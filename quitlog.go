package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// quitLog records how long each step of quitting takes, to find the cause of
// an occasional slow exit. It keeps the last runs in a small text file.
type quitLog struct {
	mu    sync.Mutex
	path  string
	begin time.Time
	steps []string
}

const quitLogRuns = 20

func newQuitLog(path string) *quitLog { return &quitLog{path: path} }

// start marks the user's close request; marks are relative to it.
func (q *quitLog) start() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.begin.IsZero() {
		q.begin = time.Now()
	}
}

func (q *quitLog) mark(step string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.begin.IsZero() {
		q.begin = time.Now()
	}
	q.steps = append(q.steps, fmt.Sprintf("%s +%dms", step, time.Since(q.begin).Milliseconds()))
}

func (q *quitLog) write() {
	q.mu.Lock()
	line := q.begin.Format(time.RFC3339) + " " + strings.Join(q.steps, ", ")
	q.mu.Unlock()
	old, _ := os.ReadFile(q.path)
	lines := strings.Split(strings.TrimSpace(string(old)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	lines = append(lines, line)
	if len(lines) > quitLogRuns {
		lines = lines[len(lines)-quitLogRuns:]
	}
	_ = os.WriteFile(q.path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
