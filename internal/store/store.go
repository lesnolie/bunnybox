// Package store owns settings.db. It uses MyGo's native SQLite library, a
// dedicated writer goroutine, WAL and a bounded pool of read-only connections.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"github.com/egoist/mygo/plugins/sqlite"
)

type request struct {
	fn   func(*sqlite.DB) error
	done chan error
}
type Store struct {
	mu      sync.RWMutex
	closed  bool
	writer  *sqlite.DB
	readers chan *sqlite.DB
	writes  chan request
	stopped chan struct{}
}

func Open(ctx context.Context, directory string) (*Store, error) {
	if !filepath.IsAbs(directory) {
		return nil, errors.New("data directory must be absolute")
	}
	if e := secureDirectory(directory); e != nil {
		return nil, e
	}
	file := filepath.Join(directory, "settings.db")
	if e := rejectLinks(file); e != nil {
		return nil, e
	}
	w, e := sqlite.Open(ctx, file, sqlite.OpenOptions{})
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*Store, error) { w.Close(); return nil, e }
	if _, e = w.Query(ctx, "PRAGMA journal_mode=WAL"); e != nil {
		return fail(e)
	}
	if _, e = w.Execute(ctx, "PRAGMA synchronous=FULL"); e != nil {
		return fail(e)
	}
	if e = migrate(ctx, w); e != nil {
		return fail(e)
	}
	s := &Store{writer: w, readers: make(chan *sqlite.DB, 2), writes: make(chan request), stopped: make(chan struct{})}
	for range 2 {
		r, e := sqlite.Open(ctx, file, sqlite.OpenOptions{ReadOnly: true})
		if e != nil {
			for len(s.readers) > 0 {
				(<-s.readers).Close()
			}
			return fail(e)
		}
		s.readers <- r
	}
	go func() {
		defer close(s.stopped)
		for req := range s.writes {
			req.done <- req.fn(w)
		}
	}()
	return s, nil
}
func (s *Store) Write(ctx context.Context, fn func(*sqlite.DB) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return sqlite.ErrClosed
	}
	req := request{fn, make(chan error, 1)}
	select {
	case s.writes <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	// Once accepted, wait for the actual outcome: callers may need to compensate
	// filesystem changes and must never mistake a successful commit for cancel.
	return <-req.done
}
func (s *Store) Query(ctx context.Context, sql string, args ...any) (sqlite.Rows, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return sqlite.Rows{}, sqlite.ErrClosed
	}
	var r *sqlite.DB
	select {
	case r = <-s.readers:
	case <-ctx.Done():
		return sqlite.Rows{}, ctx.Err()
	}
	defer func() { s.readers <- r }()
	return r.Query(ctx, sql, args...)
}
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.writes)
	<-s.stopped
	e := s.writer.Close()
	for len(s.readers) > 0 {
		e = errors.Join(e, (<-s.readers).Close())
	}
	return e
}

// Preference returns fallback on missing, unreadable or malformed data.
// Strings are raw UTF-8; all other values use JSON. nil deletes the key.
func Preference[T any](ctx context.Context, s *Store, name string, fallback T) T {
	rows, e := s.Query(ctx, "SELECT data FROM preferences WHERE name=?", name)
	if e != nil || len(rows.Values) == 0 {
		return fallback
	}
	b, ok := rows.Values[0][0].([]byte)
	if !ok {
		return fallback
	}
	if _, ok := any(fallback).(string); ok {
		if !utf8.Valid(b) {
			return fallback
		}
		return any(string(b)).(T)
	}
	var v T
	if json.Unmarshal(b, &v) != nil {
		return fallback
	}
	return v
}
func (s *Store) SetPreference(ctx context.Context, name string, value any) error {
	var b []byte
	var e error
	if v, ok := value.(string); ok {
		if !utf8.ValidString(v) {
			return errors.New("preference string must be UTF-8")
		}
		b = []byte(v)
	} else if value != nil {
		b, e = json.Marshal(value)
		if e != nil {
			return e
		}
	}
	return s.Write(ctx, func(db *sqlite.DB) error {
		if value == nil {
			_, e := db.Execute(ctx, "DELETE FROM preferences WHERE name=?", name)
			return e
		}
		_, e := db.Execute(ctx, "INSERT OR REPLACE INTO preferences(name,data) VALUES(?,?)", name, b)
		return e
	})
}
func migrate(ctx context.Context, db *sqlite.DB) error {
	if _, e := db.Execute(ctx, "CREATE TABLE IF NOT EXISTS migrations(name TEXT PRIMARY KEY NOT NULL)"); e != nil {
		return e
	}
	migrations := []struct {
		name string
		sql  []string
	}{
		{"initialize", []string{`CREATE TABLE profiles(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL UNIQUE,"order" INTEGER NOT NULL,type INTEGER NOT NULL DEFAULT 0,path TEXT NOT NULL,remoteURL TEXT,autoUpdate INTEGER NOT NULL DEFAULT 0,lastUpdated DATETIME)`, `CREATE TABLE preferences(name TEXT PRIMARY KEY ON CONFLICT REPLACE NOT NULL,data BLOB)`}},
		{"add_auto_update_interval", []string{`ALTER TABLE profiles ADD COLUMN autoUpdateInterval INTEGER NOT NULL DEFAULT 0`}},
		{"add_remote_servers", []string{`CREATE TABLE remote_servers(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL,"order" INTEGER NOT NULL,url TEXT NOT NULL,secret TEXT NOT NULL DEFAULT '')`}},
	}
	for _, m := range migrations {
		rows, e := db.Query(ctx, "SELECT name FROM migrations WHERE name=?", m.name)
		if e != nil {
			return e
		}
		if len(rows.Values) > 0 {
			continue
		}
		st := []sqlite.Statement{}
		for _, q := range m.sql {
			st = append(st, sqlite.Statement{SQL: q})
		}
		st = append(st, sqlite.Statement{SQL: "INSERT INTO migrations(name) VALUES(?)", Args: []any{m.name}})
		if _, e = db.Transaction(ctx, st); e != nil {
			return e
		}
	}
	return nil
}
func rejectLinks(file string) error {
	for p := file; ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil && (info.Mode()&os.ModeSymlink != 0 || isReparse(info)) {
			return errors.New("data path contains a link/reparse point: " + p)
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}

// SecureDirectory is also used for the private configs subdirectory.
func SecureDirectory(path string) error { return secureDirectory(path) }
