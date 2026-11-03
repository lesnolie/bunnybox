package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func TestWindowsNamedPipeDial(t *testing.T) {
	path := fmt.Sprintf(`\\.\pipe\sing-box-transport-test.%d.%d`, os.Getpid(), time.Now().UnixNano())
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	l, err := winio.ListenPipe(path, &winio.PipeConfig{SecurityDescriptor: fmt.Sprintf("D:P(A;;GA;;;%s)", user.User.Sid.String())})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	done := make(chan error, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_, err = conn.Write([]byte("ok"))
		done <- err
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := dialPipe(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var data [2]byte
	if _, err := io.ReadFull(conn, data[:]); err != nil {
		t.Fatal(err)
	}
	if string(data[:]) != "ok" {
		t.Fatal(string(data[:]))
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
