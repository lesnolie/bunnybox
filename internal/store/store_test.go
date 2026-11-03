package store

import (
	"context"
	"errors"
	"github.com/egoist/mygo/plugins/sqlite"
	"path/filepath"
	"testing"
)

func TestPreferencesAndMigrations(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, e := Open(ctx, root)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if v := Preference(ctx, db, "selected_profile_id", int64(-1)); v != -1 {
		t.Fatal(v)
	}
	for k, v := range map[string]any{"selected_profile_id": int64(42), "app_language": "简体中文", "enabled": true, "items": []string{"a", "b"}} {
		if e = db.SetPreference(ctx, k, v); e != nil {
			t.Fatal(e)
		}
	}
	if v := Preference(ctx, db, "app_language", ""); v != "简体中文" {
		t.Fatal(v)
	}
	if !Preference(ctx, db, "enabled", false) {
		t.Fatal("bool")
	}
	if v := Preference(ctx, db, "selected_profile_id", int64(-1)); v != 42 {
		t.Fatal(v)
	}
	if e = db.SetPreference(ctx, "selected_profile_id", nil); e != nil {
		t.Fatal(e)
	}
	if v := Preference(ctx, db, "selected_profile_id", int64(-1)); v != -1 {
		t.Fatal(v)
	}
	if e = db.Write(ctx, func(w *sqlite.DB) error {
		_, e := w.Execute(ctx, "INSERT OR REPLACE INTO preferences(name,data) VALUES('broken',?)", []byte("wrong JSON"))
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if v := Preference(ctx, db, "broken", 50); v != 50 {
		t.Fatal(v)
	}
	rows, e := db.Query(ctx, "SELECT name FROM migrations")
	if e != nil || len(rows.Values) != 3 {
		t.Fatalf("migrations %v %v", rows, e)
	}
	rows, e = db.Query(ctx, "PRAGMA journal_mode")
	if e != nil || rows.Values[0][0] != "wal" {
		t.Fatalf("WAL %v %v", rows, e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Query(ctx, "SELECT 1"); !errors.Is(e, sqlite.ErrClosed) {
		t.Fatal(e)
	}
	db2, e := Open(ctx, filepath.Clean(root))
	if e != nil {
		t.Fatal(e)
	}
	defer db2.Close()
	if v := Preference(ctx, db2, "app_language", ""); v != "简体中文" {
		t.Fatal("preference did not persist")
	}
}
