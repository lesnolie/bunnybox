//go:build windows

package profile

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicReplacementFailureKeepsOriginal(t *testing.T) {
	root := t.TempDir()
	s := openTest(t, root)
	p, e := s.Create(testContext, "locked")
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(root, filepath.FromSlash(p.Path))
	path, e := windows.UTF16PtrFromString(file)
	if e != nil {
		t.Fatal(e)
	}
	// An actual Windows sharing violation prevents rename/replace; the old
	// content must survive and temporary output must be removed.
	handle, e := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.UpdateContent(testContext, p.ID, `{"changed":true}`); e == nil {
		windows.CloseHandle(handle)
		t.Fatal("replacement unexpectedly succeeded without delete sharing")
	}
	windows.CloseHandle(handle)
	if content, e := s.ReadContent(testContext, p.ID); e != nil || content != "{}" {
		t.Fatalf("original lost: %q %v", content, e)
	}
	entries, e := os.ReadDir(filepath.Join(root, "configs"))
	if e != nil {
		t.Fatal(e)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v", entries)
	}
	if e = s.UpdateContent(testContext, p.ID, `{"changed":true}`); e != nil {
		t.Fatal(e)
	}
	if content, e := s.ReadContent(testContext, p.ID); e != nil || content != `{"changed":true}` {
		t.Fatalf("replacement not persisted: %q %v", content, e)
	}
}
