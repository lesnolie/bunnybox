package main

import (
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(content), 0644); e != nil {
		t.Fatal(e)
	}
}

func TestRecursiveImportsAndMissingDependency(t *testing.T) {
	root := t.TempDir()
	m := manifest{SourceRoot: filepath.Join(root, "source"), IncludeRoot: filepath.Join(root, "include"), Entries: []string{"daemon/*.proto", "experimental/boxdd/desktop_service.proto"}}
	fixture(t, m.SourceRoot, "daemon/service.proto", `syntax="proto3"; import public "shared/one.proto"; message Service { One value=1; }`)
	fixture(t, m.SourceRoot, "shared/one.proto", `syntax="proto3"; import "shared/two.proto"; message One { Two value=1; }`)
	fixture(t, m.SourceRoot, "shared/two.proto", `syntax="proto3"; message Two { string value=1; }`)
	fixture(t, m.SourceRoot, "experimental/boxdd/desktop_service.proto", `syntax="proto3"; // import "fake.proto";
 message Desktop { string value=1; }`)
	if e := os.MkdirAll(m.IncludeRoot, 0755); e != nil {
		t.Fatal(e)
	}
	entry, e := entries(m)
	if e != nil {
		t.Fatal(e)
	}
	ds, _, _, e := resolve(m, entry, root)
	if e != nil {
		t.Fatal(e)
	}
	deps, e := dependencies(m, ds)
	if e != nil {
		t.Fatal(e)
	}
	if len(deps) != 4 {
		t.Fatalf("got %d dependencies; want all four recursive files", len(deps))
	}
	if len(deps[0].Imports) != 1 || deps[0].Imports[0] != "shared/one.proto" {
		t.Fatalf("public import missing: %+v", deps[0])
	}
	before := deps[1].SHA256
	fixture(t, m.SourceRoot, "experimental/boxdd/desktop_service.proto", `syntax="proto3"; message Desktop { int64 changed=1; }`)
	ds, _, _, e = resolve(m, entry, root)
	if e != nil {
		t.Fatal(e)
	}
	deps, e = dependencies(m, ds)
	if e != nil {
		t.Fatal(e)
	}
	if deps[1].SHA256 == before {
		t.Fatal("desktop-only change was not detected")
	}
	if e = os.Remove(filepath.Join(m.SourceRoot, "shared/two.proto")); e != nil {
		t.Fatal(e)
	}
	if _, _, _, e = resolve(m, entry, root); e == nil {
		t.Fatal("missing recursive dependency must fail closed")
	}
}

func TestEntryAdditionsAndRemovals(t *testing.T) {
	m := manifest{SourceRoot: t.TempDir(), Entries: []string{"daemon/*.proto"}}
	fixture(t, m.SourceRoot, "daemon/a.proto", "")
	first, e := entries(m)
	if e != nil {
		t.Fatal(e)
	}
	fixture(t, m.SourceRoot, "daemon/b.proto", "")
	added, e := entries(m)
	if e != nil {
		t.Fatal(e)
	}
	if len(first) != 1 || len(added) != 2 {
		t.Fatal("glob did not recognize added protocol")
	}
	if e = os.Remove(filepath.Join(m.SourceRoot, "daemon/a.proto")); e != nil {
		t.Fatal(e)
	}
	removed, e := entries(m)
	if e != nil {
		t.Fatal(e)
	}
	if len(removed) != 1 || removed[0] != "daemon/b.proto" {
		t.Fatalf("unexpected removed entries: %v", removed)
	}
	if e = os.Remove(filepath.Join(m.SourceRoot, "daemon/b.proto")); e != nil {
		t.Fatal(e)
	}
	if _, e = entries(m); e == nil {
		t.Fatal("empty mandatory entry pattern must fail closed")
	}
}

func TestDesktopPackageMapping(t *testing.T) {
	m := manifest{Mappings: map[string]string{"experimental/boxdd/*": "singboxwin.local/client/internal/rpc/gen/desktop;desktop"}}
	got, e := mapping(m, "experimental/boxdd/desktop_service.proto")
	if e != nil || got != "singboxwin.local/client/internal/rpc/gen/desktop;desktop" {
		t.Fatalf("desktop main package not remapped: %q %v", got, e)
	}
	if _, e = mapping(m, "new/import.proto"); e == nil {
		t.Fatal("unmapped new package must fail closed")
	}
}
