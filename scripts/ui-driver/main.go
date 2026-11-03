// Command ui-driver bridges stdin/stdout to MyGo's local automation socket.
// Some Windows Python builds lack AF_UNIX; Go supports this transport natively.
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: ui-driver socket-path")
		os.Exit(2)
	}
	var conn net.Conn
	var err error
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, err = net.DialTimeout("unix", os.Args[1], time.Second)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()
	input := bufio.NewScanner(os.Stdin)
	input.Buffer(make([]byte, 4096), 2*1024*1024)
	reader := bufio.NewReader(conn)
	for input.Scan() {
		conn.SetDeadline(time.Now().Add(25 * time.Second))
		if _, err = fmt.Fprintln(conn, input.Text()); err != nil {
			break
		}
		var line string
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		fmt.Print(line)
	}
	if err == nil {
		err = input.Err()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
