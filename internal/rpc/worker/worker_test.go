package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The subprocess emulates only stdout/stdin lifecycle, never official Windows
// authentication. No service, driver, signing key, or upstream patch is needed.
func TestMain(m *testing.M) {
	if mode := os.Getenv("SINGBOX_TEST_CHILD"); mode != "" {
		switch mode {
		case "good":
			fmt.Println("READY")
			_, _ = io.Copy(io.Discard, os.Stdin)
		case "slow":
			time.Sleep(75 * time.Millisecond)
			fmt.Println("READY")
			_, _ = io.Copy(io.Discard, os.Stdin)
		case "ignores-eof":
			fmt.Println("READY")
			time.Sleep(time.Minute)
		case "bad":
			fmt.Fprintln(os.Stderr, "signature rejected")
			fmt.Println("NOT_READY")
			os.Exit(7)
		case "timeout":
			time.Sleep(time.Minute)
		case "crash":
			fmt.Println("READY")
			time.Sleep(100 * time.Millisecond)
			fmt.Fprint(os.Stderr, strings.Repeat("x", 70000)+"last diagnostic")
			os.Exit(9)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func testManager(t *testing.T, mode string) (*Manager, *atomic.Int32) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(Config{Executable: exe, ReadyTimeout: 250 * time.Millisecond, CloseTimeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	count := new(atomic.Int32)
	m.command = func(path string, args ...string) *exec.Cmd {
		count.Add(1)
		if path != exe {
			t.Errorf("unexpected executable %s", path)
		}
		if len(args) != 7 || args[0] != "worker" || args[1] != "--socket" || args[3] != "--parent-pid" || args[4] != fmt.Sprint(os.Getpid()) || args[5] != "--daemon-relay-socket" {
			t.Errorf("invalid worker arguments: %v", args)
		}
		if args[2] == args[6] {
			t.Error("pipe names must differ")
		}
		cmd := exec.Command(exe)
		cmd.Env = append(os.Environ(), "SINGBOX_TEST_CHILD="+mode)
		return cmd
	}
	t.Cleanup(func() { _ = m.Close() })
	return m, count
}

func TestConcurrentEnsureAndEOF(t *testing.T) {
	m, count := testManager(t, "good")
	var wg sync.WaitGroup
	processes := make(chan *Process, 24)
	for range 24 {
		wg.Go(func() {
			p, err := m.Ensure(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			processes <- p
		})
	}
	wg.Wait()
	close(processes)
	var first *Process
	for p := range processes {
		if first == nil {
			first = p
		}
		if p != first {
			t.Error("not single flight")
		}
	}
	if count.Load() != 1 {
		t.Fatalf("launched %d children", count.Load())
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.Done():
	case <-time.After(time.Second):
		t.Fatal("worker did not exit after EOF")
	}
	var exit *ExitError
	if !errors.As(first.ExitError(), &exit) || exit.Code != 0 {
		t.Fatalf("not a clean stdin EOF exit: %v", first.ExitError())
	}
	if _, err := m.Ensure(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestReadinessFailures(t *testing.T) {
	for _, mode := range []string{"bad", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			m, _ := testManager(t, mode)
			_, err := m.Ensure(context.Background())
			if err == nil {
				t.Fatal("startup succeeded")
			}
			if mode == "bad" && !strings.Contains(err.Error(), "signature rejected") {
				t.Fatal(err)
			}
			if mode == "timeout" && !strings.Contains(err.Error(), "timeout") {
				t.Fatal(err)
			}
		})
	}
}

func TestCrashTailAndRestart(t *testing.T) {
	m, count := testManager(t, "crash")
	p, err := m.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("no exit")
	}
	var exit *ExitError
	if !errors.As(p.ExitError(), &exit) || exit.Code != 9 || len(exit.Stderr) != 64*1024 || !strings.HasSuffix(exit.Stderr, "last diagnostic") {
		t.Fatalf("wrong diagnostic: %v", p.ExitError())
	}
	if count.Load() != 1 {
		t.Fatal("eager restart")
	}
	next, err := m.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if next == p || count.Load() != 2 {
		t.Fatal("worker was not restarted on demand")
	}
}

func TestCloseDuringStartup(t *testing.T) {
	m, _ := testManager(t, "timeout")
	done := make(chan error, 1)
	go func() { _, err := m.Ensure(context.Background()); done <- err }()
	// Wait until startup has been registered without a timing assumption.
	for {
		m.mu.Lock()
		starting := m.starting != nil
		m.mu.Unlock()
		if starting {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestCanceledCallerDoesNotCancelSharedStartup(t *testing.T) {
	m, count := testManager(t, "slow")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := m.Ensure(ctx); done <- err }()
	for {
		m.mu.Lock()
		starting := m.starting != nil
		m.mu.Unlock()
		if starting {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := m.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 {
		t.Fatal("caller cancellation restarted shared launch")
	}
}

func TestCloseKillsChildIgnoringEOF(t *testing.T) {
	m, _ := testManager(t, "ignores-eof")
	p, err := m.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("Close returned before child termination")
	}
	var exit *ExitError
	if !errors.As(p.ExitError(), &exit) || exit.Code == 0 {
		t.Fatal("child ignoring EOF was not killed")
	}
}

func TestPipeNames(t *testing.T) {
	pattern := regexp.MustCompile(`^\\\\\.\\pipe\\sing-box-worker\.[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 100 {
		name, err := pipeName()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(name) || seen[name] {
			t.Fatal(name)
		}
		seen[name] = true
	}
}
