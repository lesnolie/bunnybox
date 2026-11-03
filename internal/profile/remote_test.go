package profile

import (
	"errors"
	"testing"
	"time"
)

func TestRemoteProfilesFollowApple(t *testing.T) {
	root := t.TempDir()
	s := openTest(t, root)
	at := time.Date(2026, 11, 3, 8, 30, 0, 123e6, time.UTC)
	if _, e := s.Add(testContext, New{Name: "订阅", Type: TypeRemote, Content: "{}"}); e == nil {
		t.Fatal("a remote profile needs a URL")
	}
	p, e := s.Add(testContext, New{Name: "订阅", Type: TypeRemote, Content: `{"log":{}}`, RemoteURL: "https://example.com/a.json", AutoUpdate: true, AutoUpdateInterval: 60, LastUpdated: at})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.UpdateContent(testContext, p.ID, "{}"); !errors.Is(e, ErrRemoteContent) {
		t.Fatal("remote content is read-only", e)
	}
	later := at.Add(time.Hour)
	if changed, e := s.UpdateRemote(testContext, p.ID, `{"log":{}}`, later); e != nil || changed {
		t.Fatal("same content must not count as changed", changed, e)
	}
	if changed, e := s.UpdateRemote(testContext, p.ID, `{"dns":{}}`, later); e != nil || !changed {
		t.Fatal("new content", changed, e)
	}
	if _, e = s.Save(testContext, p.ID, Settings{Name: "新名", RemoteURL: "https://example.com/b.json", AutoUpdate: false, AutoUpdateInterval: 30}); e != nil {
		t.Fatal(e)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s = openTest(t, root)
	got, e := s.find(testContext, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	want := Profile{ID: p.ID, Name: "新名", Order: p.Order, Type: TypeRemote, Path: p.Path, RemoteURL: "https://example.com/b.json", AutoUpdateInterval: 30, LastUpdated: later}
	if got != want {
		t.Fatalf("persisted %+v\nwant %+v", got, want)
	}
	if content, _ := s.ReadContent(testContext, p.ID); content != `{"dns":{}}` {
		t.Fatal(content)
	}
	local, _ := s.Create(testContext, "本地")
	if _, e = s.UpdateRemote(testContext, local.ID, "{}", later); e == nil {
		t.Fatal("local profiles have nothing to update")
	}
	if saved, e := s.Save(testContext, local.ID, Settings{Name: "本地2", RemoteURL: "ignored"}); e != nil || saved.RemoteURL != "" {
		t.Fatal("local profiles keep no URL", saved, e)
	}
}
