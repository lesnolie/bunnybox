// Package profile stores local and remote JSON profiles in the Apple client's
// layout. It never touches the network: callers fetch remote content and pass
// it in. Validation is injected from the official RPC.
package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/plugins/sqlite"
	"singboxwin.local/client/internal/store"
)

var ErrNotFound = errors.New("profile not found")
var ErrNameRequired = errors.New("profile name is required")

type Checker interface {
	CheckConfig(context.Context, string) error
}
type CheckFunc func(context.Context, string) error

func (f CheckFunc) CheckConfig(ctx context.Context, c string) error { return f(ctx, c) }

// Profile types of the Apple client; iCloud has no Windows counterpart.
const (
	TypeLocal  int64 = 0
	TypeRemote int64 = 2
)

// ErrRemoteContent rejects editing a remote profile, which Apple shows read-only.
var ErrRemoteContent = errors.New("远程配置的内容只读，请更新订阅 / remote profile content is read-only")

type Profile struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Order int64  `json:"order"`
	Type  int64  `json:"type"`
	Path  string `json:"path"`
	// Remote profiles only. AutoUpdateInterval is in minutes, 0 is the default.
	RemoteURL          string    `json:"remoteURL,omitempty"`
	AutoUpdate         bool      `json:"autoUpdate,omitempty"`
	AutoUpdateInterval int64     `json:"autoUpdateInterval,omitempty"`
	LastUpdated        time.Time `json:"lastUpdated,omitzero"`
}

// New describes a profile to create; Content is its JSON configuration.
type New struct {
	Name, Content      string
	Type               int64
	RemoteURL          string
	AutoUpdate         bool
	AutoUpdateInterval int64
	LastUpdated        time.Time
}

// Settings are the fields of Apple's Edit Profile form.
type Settings struct {
	Name               string
	RemoteURL          string
	AutoUpdate         bool
	AutoUpdateInterval int64
}

// GRDB's date format, so the database matches the Apple client's.
const dateLayout = "2006-01-02 15:04:05.000"

type Service struct {
	mu      sync.Mutex
	db      *store.Store
	root    string
	checker Checker
	closed  bool
}

// Open never chooses APPDATA: the application's validated layout supplies root.
// A process must own only one Service for a directory (the app is single-instance).
func Open(ctx context.Context, root string, checker Checker) (*Service, error) {
	if checker == nil {
		return nil, errors.New("official CheckConfig validator is required")
	}
	db, e := store.Open(ctx, root)
	if e != nil {
		return nil, e
	}
	s := &Service{db: db, root: root, checker: checker}
	if e = store.SecureDirectory(filepath.Join(root, "configs")); e != nil {
		db.Close()
		return nil, e
	}
	if e = s.recover(ctx); e != nil {
		db.Close()
		return nil, e
	}
	return s, nil
}
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.db.Close()
}
func (s *Service) ready(ctx context.Context) error {
	if s.closed {
		return sqlite.ErrClosed
	}
	if _, e := os.Lstat(s.journalPath()); e == nil {
		return errors.New("pending profile operation requires closing and reopening service for recovery")
	} else if !os.IsNotExist(e) {
		return e
	}
	return ctx.Err()
}
func (s *Service) List(ctx context.Context) ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return nil, e
	}
	return s.list(ctx)
}
func (s *Service) list(ctx context.Context) ([]Profile, error) {
	rows, e := s.db.Query(ctx, `SELECT id,name,"order",type,path,remoteURL,autoUpdate,autoUpdateInterval,lastUpdated FROM profiles ORDER BY "order",id`)
	if e != nil {
		return nil, e
	}
	result := []Profile{}
	for _, r := range rows.Values {
		if len(r) != 9 {
			return nil, errors.New("invalid stored profile record")
		}
		id, idOK := r[0].(int64)
		name, nameOK := r[1].(string)
		order, orderOK := r[2].(int64)
		kind, kindOK := r[3].(int64)
		path, pathOK := r[4].(string)
		auto, autoOK := r[6].(int64)
		interval, intervalOK := r[7].(int64)
		if !idOK || !nameOK || !orderOK || !kindOK || !pathOK || !autoOK || !intervalOK || id <= 0 {
			return nil, errors.New("invalid stored profile fields")
		}
		p := Profile{ID: id, Name: name, Order: order, Type: kind, Path: path, AutoUpdate: auto != 0, AutoUpdateInterval: interval}
		if url, ok := r[5].(string); ok {
			p.RemoteURL = url
		}
		if text, ok := r[8].(string); ok && text != "" {
			if p.LastUpdated, e = time.ParseInLocation(dateLayout, text, time.UTC); e != nil {
				return nil, fmt.Errorf("invalid stored update time for id %d", p.ID)
			}
		}
		if p.Type != TypeLocal && p.Type != TypeRemote {
			return nil, fmt.Errorf("unsupported profile type %d for id %d", p.Type, p.ID)
		}
		if p.Path != fmt.Sprintf("configs/config_%d.json", p.ID) {
			return nil, fmt.Errorf("invalid stored profile path for id %d", p.ID)
		}
		result = append(result, p)
	}
	return result, nil
}
func (s *Service) find(ctx context.Context, id int64) (Profile, error) {
	list, e := s.list(ctx)
	if e != nil {
		return Profile{}, e
	}
	for _, p := range list {
		if p.ID == id {
			return p, nil
		}
	}
	return Profile{}, ErrNotFound
}
func (s *Service) unique(ctx context.Context, name string, exclude int64) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrNameRequired
	}
	if !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		return "", errors.New("profile name must be UTF-8 without NUL")
	}
	list, e := s.list(ctx)
	if e != nil {
		return "", e
	}
	used := map[string]bool{}
	for _, p := range list {
		if p.ID != exclude {
			used[p.Name] = true
		}
	}
	candidate := name
	for n := 1; used[candidate]; n++ {
		candidate = fmt.Sprintf("%s (%d)", name, n)
	}
	return candidate, nil
}
func normalize(content string) (string, error) {
	content = strings.TrimPrefix(content, "\ufeff")
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return "", errors.New("configuration must be UTF-8 without NUL")
	}
	return content, nil
}
func (s *Service) validate(ctx context.Context, content string) (string, error) {
	content, e := normalize(content)
	if e != nil {
		return "", e
	}
	if e = s.checker.CheckConfig(ctx, content); e != nil {
		return "", fmt.Errorf("check configuration: %w", e)
	}
	if e = ctx.Err(); e != nil {
		return "", e
	}
	return content, nil
}
func (s *Service) Create(ctx context.Context, name string) (Profile, error) {
	return s.ImportJSON(ctx, name, "{}")
}
func (s *Service) ImportJSON(ctx context.Context, name, content string) (Profile, error) {
	return s.Add(ctx, New{Name: name, Content: content})
}

// Add creates a local or remote profile and selects it.
func (s *Service) Add(ctx context.Context, n New) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return Profile{}, e
	}
	if e := checkRemote(n.Type, n.RemoteURL, n.AutoUpdateInterval); e != nil {
		return Profile{}, e
	}
	name, e := s.unique(ctx, n.Name, 0)
	if e != nil {
		return Profile{}, e
	}
	content, e := s.validate(ctx, n.Content)
	if e != nil {
		return Profile{}, e
	}
	rows, e := s.db.Query(ctx, `SELECT COALESCE(MAX(id),0)+1,COALESCE(MAX("order"),-1)+1 FROM profiles`)
	if e != nil {
		return Profile{}, e
	}
	if len(rows.Values) != 1 || len(rows.Values[0]) != 2 {
		return Profile{}, errors.New("invalid profile sequence query result")
	}
	nextID, idOK := rows.Values[0][0].(int64)
	nextOrder, orderOK := rows.Values[0][1].(int64)
	if !idOK || !orderOK {
		return Profile{}, errors.New("invalid profile sequence values")
	}
	p := Profile{ID: nextID, Name: name, Order: nextOrder, Type: n.Type}
	if n.Type == TypeRemote {
		p.RemoteURL, p.AutoUpdate, p.AutoUpdateInterval = n.RemoteURL, n.AutoUpdate, n.AutoUpdateInterval
		if !n.LastUpdated.IsZero() {
			p.LastUpdated = n.LastUpdated.UTC().Truncate(time.Millisecond)
		}
	}
	p.Path = fmt.Sprintf("configs/config_%d.json", p.ID)
	file := filepath.Join(s.root, filepath.FromSlash(p.Path))
	if _, e = os.Lstat(file); !os.IsNotExist(e) {
		if e != nil {
			return Profile{}, e
		}
		return Profile{}, errors.New("untracked profile content exists; refusing overwrite")
	}
	pending := journal{Action: "create", ID: p.ID}
	if e = s.writeJournal(pending); e != nil {
		return Profile{}, e
	}
	if e = atomicWrite(file, []byte(content)); e != nil {
		_ = s.clearJournal()
		return Profile{}, e
	}
	e = s.db.Write(ctx, func(db *sqlite.DB) error {
		_, e := db.Transaction(ctx, []sqlite.Statement{
			{SQL: `INSERT INTO profiles(id,name,"order",type,path,remoteURL,autoUpdate,autoUpdateInterval,lastUpdated) VALUES(?,?,?,?,?,?,?,?,?)`,
				Args: []any{p.ID, p.Name, p.Order, p.Type, p.Path, nullable(p.RemoteURL), boolInt(p.AutoUpdate), p.AutoUpdateInterval, dateValue(p.LastUpdated)}}, selectedStatement(p.ID)})
		return e
	})
	if e != nil {
		cleanup := os.Remove(file)
		if cleanup == nil {
			cleanup = s.clearJournal()
		}
		return Profile{}, errors.Join(e, cleanup)
	}
	if e = s.clearJournal(); e != nil {
		return p, fmt.Errorf("profile saved but journal cleanup failed: %w", e)
	}
	return p, nil
}
func selectedStatement(id int64) sqlite.Statement {
	b, _ := json.Marshal(id)
	return sqlite.Statement{SQL: `INSERT OR REPLACE INTO preferences(name,data) VALUES('selected_profile_id',?)`, Args: []any{b}}
}
func (s *Service) ReadContent(ctx context.Context, id int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return "", e
	}
	p, e := s.find(ctx, id)
	if e != nil {
		return "", e
	}
	file := filepath.Join(s.root, filepath.FromSlash(p.Path))
	if e = regular(file); e != nil {
		return "", e
	}
	b, e := os.ReadFile(file)
	if e != nil {
		return "", e
	}
	return normalize(string(b))
}
func (s *Service) UpdateContent(ctx context.Context, id int64, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return e
	}
	p, e := s.find(ctx, id)
	if e != nil {
		return e
	}
	if p.Type == TypeRemote {
		return ErrRemoteContent
	}
	content, e = s.validate(ctx, content)
	if e != nil {
		return e
	}
	file := filepath.Join(s.root, filepath.FromSlash(p.Path))
	if e = regular(file); e != nil {
		return e
	}
	return atomicWrite(file, []byte(content))
}

// UpdateRemote stores freshly fetched content of a remote profile, as Apple's
// updateRemoteProfile does: it records the update time and rewrites the file
// only when the content changed, which it reports.
func (s *Service) UpdateRemote(ctx context.Context, id int64, content string, at time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return false, e
	}
	p, e := s.find(ctx, id)
	if e != nil {
		return false, e
	}
	if p.Type != TypeRemote {
		return false, errors.New("not a remote profile")
	}
	content, e = s.validate(ctx, content)
	if e != nil {
		return false, e
	}
	e = s.db.Write(ctx, func(db *sqlite.DB) error {
		_, e := db.Execute(ctx, "UPDATE profiles SET lastUpdated=? WHERE id=?", dateValue(at), id)
		return e
	})
	if e != nil {
		return false, e
	}
	file := filepath.Join(s.root, filepath.FromSlash(p.Path))
	if e = regular(file); e != nil {
		return false, e
	}
	if old, e := os.ReadFile(file); e == nil && string(old) == content {
		return false, nil
	}
	return true, atomicWrite(file, []byte(content))
}

// Save applies Apple's Edit Profile form; the type never changes.
func (s *Service) Save(ctx context.Context, id int64, set Settings) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return Profile{}, e
	}
	p, e := s.find(ctx, id)
	if e != nil {
		return Profile{}, e
	}
	if p.Type == TypeRemote {
		if e = checkRemote(p.Type, set.RemoteURL, set.AutoUpdateInterval); e != nil {
			return Profile{}, e
		}
		p.RemoteURL, p.AutoUpdate, p.AutoUpdateInterval = set.RemoteURL, set.AutoUpdate, set.AutoUpdateInterval
	}
	if p.Name, e = s.unique(ctx, set.Name, id); e != nil {
		return Profile{}, e
	}
	e = s.db.Write(ctx, func(db *sqlite.DB) error {
		_, e := db.Execute(ctx, "UPDATE profiles SET name=?,remoteURL=?,autoUpdate=?,autoUpdateInterval=? WHERE id=?",
			p.Name, nullable(p.RemoteURL), boolInt(p.AutoUpdate), p.AutoUpdateInterval, id)
		return e
	})
	if e != nil {
		return Profile{}, e
	}
	return p, nil
}

func checkRemote(kind int64, url string, interval int64) error {
	switch kind {
	case TypeLocal:
		return nil
	case TypeRemote:
		if strings.TrimSpace(url) == "" {
			return errors.New("缺少订阅 URL / missing URL")
		}
		if interval < 0 {
			return errors.New("自动更新间隔无效 / invalid auto update interval")
		}
		return nil
	}
	return fmt.Errorf("unsupported profile type %d", kind)
}

func nullable(text string) any {
	if text == "" {
		return nil
	}
	return text
}
func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
func dateValue(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(dateLayout)
}
func (s *Service) Rename(ctx context.Context, id int64, name string) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return Profile{}, e
	}
	p, e := s.find(ctx, id)
	if e != nil {
		return Profile{}, e
	}
	name, e = s.unique(ctx, name, id)
	if e != nil {
		return Profile{}, e
	}
	e = s.db.Write(ctx, func(db *sqlite.DB) error {
		_, e := db.Execute(ctx, "UPDATE profiles SET name=? WHERE id=?", name, id)
		return e
	})
	if e != nil {
		return Profile{}, e
	}
	p.Name = name
	return p, nil
}
func (s *Service) Select(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return e
	}
	if _, e := s.find(ctx, id); e != nil {
		return e
	}
	return s.db.SetPreference(ctx, "selected_profile_id", id)
}
func (s *Service) Selected(ctx context.Context) (*Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return nil, e
	}
	id := store.Preference(ctx, s.db, "selected_profile_id", int64(-1))
	if id < 0 {
		return nil, nil
	}
	p, e := s.find(ctx, id)
	if errors.Is(e, ErrNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	return &p, nil
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ready(ctx); e != nil {
		return e
	}
	p, e := s.find(ctx, id)
	if e != nil {
		return e
	}
	file := filepath.Join(s.root, filepath.FromSlash(p.Path))
	if e = regular(file); e != nil {
		return e
	}
	list, e := s.list(ctx)
	if e != nil {
		return e
	}
	fallback := int64(-1)
	for _, q := range list {
		if q.ID != id {
			fallback = q.ID
			break
		}
	}
	trash := file + ".delete"
	if _, e = os.Lstat(trash); !os.IsNotExist(e) {
		return errors.New("pending delete file already exists; recovery required")
	}
	if e = s.writeJournal(journal{Action: "delete", ID: id}); e != nil {
		return e
	}
	if e = os.Rename(file, trash); e != nil {
		_ = s.clearJournal()
		return e
	}
	e = s.db.Write(ctx, func(db *sqlite.DB) error {
		st := []sqlite.Statement{{SQL: "DELETE FROM profiles WHERE id=?", Args: []any{id}}}
		selected, e := storeSelected(ctx, db)
		if e != nil {
			return e
		}
		if selected == id {
			st = append(st, selectedStatement(fallback))
		}
		_, e = db.Transaction(ctx, st)
		return e
	})
	if e != nil {
		restore := os.Rename(trash, file)
		if restore == nil {
			restore = s.clearJournal()
		}
		return errors.Join(e, restore)
	}
	if e = os.Remove(trash); e != nil {
		return fmt.Errorf("profile deleted but pending content cleanup failed: %w", e)
	}
	return s.clearJournal()
}
func storeSelected(ctx context.Context, db *sqlite.DB) (int64, error) {
	r, e := db.Query(ctx, "SELECT data FROM preferences WHERE name='selected_profile_id'")
	if e != nil {
		return -1, e
	}
	if len(r.Values) == 0 {
		return -1, nil
	}
	b, ok := r.Values[0][0].([]byte)
	if !ok {
		return -1, nil
	}
	var id int64
	if json.Unmarshal(b, &id) != nil {
		return -1, nil
	}
	return id, nil
}

type journal struct {
	Action string `json:"action"`
	ID     int64  `json:"id"`
}

func (s *Service) journalPath() string { return filepath.Join(s.root, "configs", ".pending.json") }
func (s *Service) writeJournal(j journal) error {
	if _, e := os.Lstat(s.journalPath()); !os.IsNotExist(e) {
		return errors.New("pending filesystem operation requires reopening service for recovery")
	}
	b, _ := json.Marshal(j)
	return atomicWrite(s.journalPath(), b)
}
func (s *Service) clearJournal() error { return os.Remove(s.journalPath()) }

// Recovery chooses the committed SQLite state after interruption, never scans
// or removes unrelated files. A pending delete restores its content if the row
// still exists; a committed create retains its content, an uncommitted one does not.
func (s *Service) recover(ctx context.Context) error {
	if _, e := os.Lstat(s.journalPath()); os.IsNotExist(e) {
		return nil
	} else if e != nil {
		return e
	}
	if e := regular(s.journalPath()); e != nil {
		return e
	}
	b, e := os.ReadFile(s.journalPath())
	if e != nil {
		return e
	}
	var j journal
	if e = json.Unmarshal(b, &j); e != nil {
		return e
	}
	if j.ID <= 0 || (j.Action != "create" && j.Action != "delete") {
		return errors.New("invalid pending profile operation")
	}
	rows, e := s.db.Query(ctx, "SELECT id FROM profiles WHERE id=?", j.ID)
	if e != nil {
		return e
	}
	exists := len(rows.Values) > 0
	file := filepath.Join(s.root, "configs", fmt.Sprintf("config_%d.json", j.ID))
	if j.Action == "create" {
		if !exists {
			if e = removeRegular(file); e != nil {
				return e
			}
		} else if e = regular(file); e != nil {
			return e
		}
	} else {
		trash := file + ".delete"
		_, trashErr := os.Lstat(trash)
		if trashErr != nil && !os.IsNotExist(trashErr) {
			return trashErr
		}
		if exists && trashErr == nil {
			if _, e = os.Lstat(file); !os.IsNotExist(e) {
				return errors.New("ambiguous interrupted delete: both content paths exist")
			}
			if e = regular(trash); e != nil {
				return e
			}
			if e = os.Rename(trash, file); e != nil {
				return e
			}
		} else if !exists {
			if e = removeRegular(trash); e != nil {
				return e
			}
		} else if e = regular(file); e != nil {
			return e
		}
	}
	return s.clearJournal()
}
func regular(file string) error {
	if e := store.SecureDirectory(filepath.Dir(file)); e != nil {
		return e
	}
	i, e := os.Lstat(file)
	if e != nil {
		return e
	}
	if !i.Mode().IsRegular() {
		return errors.New("profile content is not a regular file")
	}
	return nil
}
func removeRegular(file string) error {
	if _, e := os.Lstat(file); os.IsNotExist(e) {
		return nil
	}
	if e := regular(file); e != nil {
		return e
	}
	return os.Remove(file)
}
func atomicWrite(file string, b []byte) error {
	if e := store.SecureDirectory(filepath.Dir(file)); e != nil {
		return e
	}
	tmp, e := os.CreateTemp(filepath.Dir(file), ".write-*.tmp")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e != nil {
		tmp.Close()
		return e
	}
	if e = tmp.Sync(); e != nil {
		tmp.Close()
		return e
	}
	if e = tmp.Close(); e != nil {
		return e
	}
	return replaceFile(name, file)
}

// PreferenceString reads an application preference stored in the Apple
// client's preferences table; fallback covers missing or invalid data.
func (s *Service) PreferenceString(ctx context.Context, name, fallback string) string {
	return store.Preference(ctx, s.db, name, fallback)
}

// PreferenceBool is PreferenceString for a boolean preference.
func (s *Service) PreferenceBool(ctx context.Context, name string, fallback bool) bool {
	return store.Preference(ctx, s.db, name, fallback)
}

// PreferenceStrings is PreferenceString for a list of strings; nil if unset.
func (s *Service) PreferenceStrings(ctx context.Context, name string) []string {
	return store.Preference[[]string](ctx, s.db, name, nil)
}

// SetPreference stores an application preference; nil deletes it.
func (s *Service) SetPreference(ctx context.Context, name string, value any) error {
	if name == "selected_profile_id" {
		return errors.New("selected_profile_id is managed by Select")
	}
	return s.db.SetPreference(ctx, name, value)
}
