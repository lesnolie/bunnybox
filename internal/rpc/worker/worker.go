// Package worker owns the official, unmodified daemon's child worker process.
package worker

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrClosed = errors.New("worker manager is closed")

type Config struct {
	Executable   string
	ReadyTimeout time.Duration // Default: ten seconds; cannot exceed the protocol limit.
	CloseTimeout time.Duration // Grace period after stdin EOF; default: two seconds.
}

type Manager struct {
	mu       sync.Mutex
	config   Config
	process  *Process
	starting *startup
	closed   bool
	ctx      context.Context
	cancel   context.CancelFunc
	command  func(string, ...string) *exec.Cmd
}

// Each waiter retains its own attempt result even if a later caller has already
// begun replacing a child that exited immediately after READY.
type startup struct {
	done    chan struct{}
	process *Process
	err     error
}

type Process struct {
	ApplicationPipe string
	DaemonPipe      string
	cmd             *exec.Cmd
	stdin           io.WriteCloser
	tail            *tailBuffer
	done            chan struct{}
	mu              sync.Mutex
	err             error
	stop            sync.Once
}

// ExitError preserves the exit code and bounded diagnostic tail from the worker.
type ExitError struct {
	Code   int
	Stderr string
	Cause  error
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("worker exited (code %d): %v; stderr: %s", e.Code, e.Cause, strings.TrimSpace(e.Stderr))
}
func (e *ExitError) Unwrap() error { return e.Cause }

func InstalledExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "resources", "daemon", "sing-box-daemon.exe"), nil
}

func New(config Config) (*Manager, error) {
	if !filepath.IsAbs(config.Executable) {
		return nil, errors.New("worker executable must be an absolute installed path")
	}
	if config.ReadyTimeout <= 0 || config.ReadyTimeout > 10*time.Second {
		config.ReadyTimeout = 10 * time.Second
	}
	if config.CloseTimeout <= 0 {
		config.CloseTimeout = 2 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{config: config, ctx: ctx, cancel: cancel, command: exec.Command}, nil
}

// Ensure single-flights startup. Canceling one caller does not cancel other callers.
// A crashed worker is restarted only by a subsequent Ensure call.
func (m *Manager) Ensure(ctx context.Context) (*Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrClosed
	}
	if p := m.process; p != nil {
		select {
		case <-p.done:
			m.process = nil
		default:
			m.mu.Unlock()
			return p, nil
		}
	}
	if m.starting == nil {
		m.starting = &startup{done: make(chan struct{})}
		go m.start(m.starting)
	}
	wait := m.starting
	m.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-wait.done:
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	if wait.err != nil {
		return nil, wait.err
	}
	select {
	case <-wait.process.done:
		return nil, wait.process.ExitError()
	default:
	}
	return wait.process, nil
}

func (m *Manager) start(wait *startup) {
	p, err := m.launch()
	m.mu.Lock()
	closed := m.closed
	if !closed {
		m.process = p
	}
	m.mu.Unlock()
	if closed && p != nil {
		p.close(m.config.CloseTimeout)
	}
	m.mu.Lock()
	wait.process, wait.err = p, err
	m.starting = nil
	close(wait.done)
	m.mu.Unlock()
}

func (m *Manager) launch() (*Process, error) {
	p1, err := pipeName()
	if err != nil {
		return nil, err
	}
	p2, err := pipeName()
	if err != nil {
		return nil, err
	}
	for p2 == p1 {
		p2, err = pipeName()
		if err != nil {
			return nil, err
		}
	}
	// No shell, helper, elevation, or alternate parent process is involved.
	cmd := m.command(m.config.Executable, "worker", "--socket", p1, "--parent-pid", strconv.Itoa(os.Getpid()), "--daemon-relay-socket", p2)
	hideWindow(cmd)
	p := &Process{ApplicationPipe: p1, DaemonPipe: p2, cmd: cmd, tail: &tailBuffer{}, done: make(chan struct{})}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	p.stdin = stdin
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	cmd.Stderr = p.tail
	if err = cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("start worker: %w", err)
	}
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.err = &ExitError{Code: cmd.ProcessState.ExitCode(), Stderr: p.tail.String(), Cause: err}
		p.mu.Unlock()
		stdin.Close()
		close(p.done)
	}()
	ready := make(chan error, 1)
	go func() {
		r := bufio.NewReaderSize(stdout, 4096)
		line, err := r.ReadSlice('\n')
		if err == nil && string(line) != "READY\n" && string(line) != "READY\r\n" {
			err = fmt.Errorf("unexpected worker readiness line: %q", line)
		}
		ready <- err
		if err == nil {
			_, _ = io.Copy(io.Discard, r)
		}
	}()
	timer := time.NewTimer(m.config.ReadyTimeout)
	defer timer.Stop()
	select {
	case err = <-ready:
		if err == nil {
			select {
			case <-p.done:
				return nil, p.ExitError()
			default:
				return p, nil
			}
		}
	case <-p.done:
		return nil, p.ExitError()
	case <-timer.C:
		err = errors.New("worker READY timeout")
	case <-m.ctx.Done():
		err = ErrClosed
	}
	_ = cmd.Process.Kill()
	<-p.done
	return nil, fmt.Errorf("worker startup: %w; %v", err, p.ExitError())
}

func (p *Process) Done() <-chan struct{} { return p.done }

// ExitError is nil while the child is alive. Even code zero is an unexpected exit
// unless the manager was explicitly closed.
func (p *Process) ExitError() error { p.mu.Lock(); defer p.mu.Unlock(); return p.err }
func (p *Process) close(timeout time.Duration) {
	p.stop.Do(func() {
		_ = p.stdin.Close()
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-p.done:
		case <-timer.C:
			_ = p.cmd.Process.Kill()
			<-p.done
		}
	})
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	m.cancel()
	p, wait := m.process, m.starting
	m.mu.Unlock()
	if p != nil {
		p.close(m.config.CloseTimeout)
	}
	if wait != nil {
		<-wait.done
	}
	return nil
}

func pipeName() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(`\\.\pipe\sing-box-worker.%x-%x-%x-%x-%x`, b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

type tailBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *tailBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(data)
	const limit = 64 * 1024
	if len(data) >= limit {
		b.data = append(b.data[:0], data[len(data)-limit:]...)
	} else {
		if excess := len(b.data) + len(data) - limit; excess > 0 {
			copy(b.data, b.data[excess:])
			b.data = b.data[:len(b.data)-excess]
		}
		b.data = append(b.data, data...)
	}
	return n, nil
}
func (b *tailBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return string(b.data) }
