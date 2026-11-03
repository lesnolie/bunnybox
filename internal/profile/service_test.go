package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"
)

var testContext = context.Background()

func testChecker(context.Context, string) error { return nil }
func openTest(t *testing.T, root string) *Service {
	t.Helper()
	s, e := Open(testContext, root, CheckFunc(testChecker))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestLocalLifecyclePersistsAndSelectionFallsBack(t *testing.T) {
	root := t.TempDir()
	s := openTest(t, root)
	first, e := s.Create(testContext, "工作")
	if e != nil {
		t.Fatal(e)
	}
	second, e := s.ImportJSON(testContext, "工作", "\ufeff{\"log\":{}}")
	if e != nil {
		t.Fatal(e)
	}
	if first.ID != 1 || second.ID != 2 || second.Name != "工作 (1)" || second.Order != 1 {
		t.Fatalf("unexpected profiles: %+v %+v", first, second)
	}
	b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(second.Path)))
	if e != nil || string(b) != "{\"log\":{}}" {
		t.Fatalf("UTF-8 without BOM: %q %v", b, e)
	}
	selected, e := s.Selected(testContext)
	if e != nil || selected == nil || selected.ID != second.ID {
		t.Fatalf("selected: %+v %v", selected, e)
	}
	renamed, e := s.Rename(testContext, second.ID, "工作")
	if e != nil || renamed.Name != "工作 (1)" {
		t.Fatalf("rename: %+v %v", renamed, e)
	}
	if e = s.UpdateContent(testContext, first.ID, "{\"中文\":true}"); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s = openTest(t, root)
	content, e := s.ReadContent(testContext, first.ID)
	if e != nil || content != "{\"中文\":true}" {
		t.Fatalf("persisted: %q %v", content, e)
	}
	if e = s.Delete(testContext, second.ID); e != nil {
		t.Fatal(e)
	}
	selected, e = s.Selected(testContext)
	if e != nil || selected == nil || selected.ID != first.ID {
		t.Fatalf("fallback: %+v %v", selected, e)
	}
	if _, e = os.Stat(filepath.Join(root, filepath.FromSlash(second.Path))); !os.IsNotExist(e) {
		t.Fatal("deleted content remains")
	}
	if e = s.Delete(testContext, first.ID); e != nil {
		t.Fatal(e)
	}
	if selected, e = s.Selected(testContext); e != nil || selected != nil {
		t.Fatalf("empty selection: %+v %v", selected, e)
	}
	if _, e = s.ReadContent(testContext, first.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
}
func TestValidatorAndWriteFailuresDoNotChangeState(t *testing.T) {
	root := t.TempDir()
	reject := false
	s, e := Open(testContext, root, CheckFunc(func(_ context.Context, c string) error {
		if reject {
			return errors.New("official validator rejected")
		}
		return nil
	}))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	p, e := s.Create(testContext, "original")
	if e != nil {
		t.Fatal(e)
	}
	reject = true
	if e = s.UpdateContent(testContext, p.ID, "invalid"); e == nil {
		t.Fatal("validation not enforced")
	}
	if _, e = s.ImportJSON(testContext, "bad", "{}"); e == nil {
		t.Fatal("import validation not enforced")
	}
	reject = false
	if _, e = s.ImportJSON(testContext, "bad", string([]byte{0xff})); e == nil {
		t.Fatal("invalid UTF8 accepted")
	}
	if _, e = s.Create(testContext, "  "); !errors.Is(e, ErrNameRequired) {
		t.Fatal(e)
	}
	// A real SQLite trigger rejects insertion after content has been written.
	if e = s.db.Write(testContext, func(db *sqlite.DB) error {
		_, e := db.Execute(testContext, `CREATE TRIGGER fail_insert BEFORE INSERT ON profiles BEGIN SELECT RAISE(ABORT,'injected insert failure'); END`)
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Create(testContext, "failed"); e == nil {
		t.Fatal("SQL failure not surfaced")
	}
	if _, e = os.Stat(filepath.Join(root, "configs/config_2.json")); !os.IsNotExist(e) {
		t.Fatal("failed insertion left orphan content")
	}
	if _, e = os.Stat(s.journalPath()); !os.IsNotExist(e) {
		t.Fatal("compensated insertion left journal")
	}
	if e = s.db.Write(testContext, func(db *sqlite.DB) error {
		_, e := db.Execute(testContext, `CREATE TRIGGER fail_delete BEFORE DELETE ON profiles BEGIN SELECT RAISE(ABORT,'injected delete failure'); END`)
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if e = s.Delete(testContext, p.ID); e == nil {
		t.Fatal("delete failure not surfaced")
	}
	if content, e := s.ReadContent(testContext, p.ID); e != nil || content != "{}" {
		t.Fatalf("failed delete lost content: %q %v", content, e)
	}
	if selected, e := s.Selected(testContext); e != nil || selected == nil || selected.ID != p.ID {
		t.Fatal("failed operation changed selected profile")
	}
	// Atomic replacement failure leaves the original file intact.
	file := filepath.Join(root, "configs/config_1.json")
	original, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = atomicWrite(filepath.Join(file, "child"), []byte("bad")); e == nil {
		t.Fatal("expected non-directory error")
	}
	current, _ := os.ReadFile(file)
	if string(original) != string(current) {
		t.Fatal("failed replacement damaged original")
	}
}
func TestConcurrentUniqueNames(t *testing.T) {
	s := openTest(t, t.TempDir())
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for range 12 {
		wg.Go(func() { _, e := s.Create(testContext, "same"); errs <- e })
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	list, e := s.List(testContext)
	if e != nil || len(list) != 12 {
		t.Fatalf("%d %v", len(list), e)
	}
	used := map[string]bool{}
	for _, p := range list {
		if used[p.Name] {
			t.Fatal("duplicate name")
		}
		used[p.Name] = true
	}
}
func TestInterruptedFileDatabaseOperationsRecover(t *testing.T) {
	for _, action := range []string{"uncommitted-create", "committed-create", "uncommitted-delete", "committed-delete"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			s := openTest(t, root)
			p, e := s.Create(testContext, "persist")
			if e != nil {
				t.Fatal(e)
			}
			file := filepath.Join(root, filepath.FromSlash(p.Path))
			j := journal{ID: p.ID}
			if strings.HasSuffix(action, "create") {
				j.Action = "create"
			} else {
				j.Action = "delete"
			}
			if e = s.writeJournal(j); e != nil {
				t.Fatal(e)
			}
			if j.Action == "delete" {
				if e = os.Rename(file, file+".delete"); e != nil {
					t.Fatal(e)
				}
			}
			removed := action == "uncommitted-create" || action == "committed-delete"
			if removed {
				if e = s.db.Write(testContext, func(db *sqlite.DB) error {
					_, e := db.Execute(testContext, "DELETE FROM profiles WHERE id=?", p.ID)
					return e
				}); e != nil {
					t.Fatal(e)
				}
			}
			s.Close()
			s = openTest(t, root)
			list, e := s.List(testContext)
			if e != nil {
				t.Fatal(e)
			}
			if removed {
				if len(list) != 0 {
					t.Fatal("uncommitted/removed metadata survived")
				}
				if _, e = os.Stat(file); !os.IsNotExist(e) {
					t.Fatal("orphan content survived")
				}
			} else {
				if len(list) != 1 {
					t.Fatal("committed metadata lost")
				}
				if content, e := s.ReadContent(testContext, p.ID); e != nil || content != "{}" {
					t.Fatalf("content not recovered: %q %v", content, e)
				}
			}
			if _, e = os.Stat(s.journalPath()); !os.IsNotExist(e) {
				t.Fatal("journal not cleared")
			}
		})
	}
}
func TestImportsDoNotInterpretNameAsPath(t *testing.T) {
	s := openTest(t, t.TempDir())
	p, e := s.Create(testContext, "../outside")
	if e != nil {
		t.Fatal(e)
	}
	if p.Path != "configs/config_1.json" {
		t.Fatal(p)
	}
	if e = s.Select(testContext, 999); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(testContext)
	cancel()
	if _, e = s.Create(ctx, "cancel"); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestRequiresOfficialValidator(t *testing.T) {
	if _, e := Open(testContext, t.TempDir(), nil); e == nil {
		t.Fatal("validator must be injected")
	}
}
func TestSQLiteFileIsReal(t *testing.T) {
	root := t.TempDir()
	s := openTest(t, root)
	if _, e := s.Create(testContext, "real"); e != nil {
		t.Fatal(e)
	}
	s.Close()
	b, e := os.ReadFile(filepath.Join(root, "settings.db"))
	if e != nil || !strings.HasPrefix(string(b), "SQLite format 3\x00") {
		t.Fatalf("not a persisted SQLite file: %v", e)
	}
}
