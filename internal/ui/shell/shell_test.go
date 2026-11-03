package shell

import (
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// complete waits for the background publication, never for an arbitrary delay.
func complete(t *testing.T, s *Shell, tt *ui.Tester) {
	t.Helper()
	if !s.busy {
		return
	}
	select {
	case r := <-s.results:
		s.results <- r
	case <-t.Context().Done():
		t.Fatal("operation did not publish")
	}
	tt.Frame()
}

func TestProfileWorkflow(t *testing.T) {
	var stored []Profile
	contents := map[string]string{}
	var calls []string
	next := 0
	add := func(p Profile, content string) {
		next++
		p.ID = string(rune('0' + next))
		for i := range stored {
			stored[i].Selected = false
		}
		p.Selected = true
		stored = append(stored, p)
		contents[p.ID] = content
	}
	index := func(id string) int {
		for i := range stored {
			if stored[i].ID == id {
				return i
			}
		}
		t.Fatal("unknown id", id)
		return -1
	}
	openName, openData := "", []byte(nil)
	cb := Callbacks{
		List: func(context.Context) ([]Profile, error) { return append([]Profile(nil), stored...), nil },
		Create: func(_ context.Context, name, json string) error {
			add(Profile{Name: name, Type: "local"}, json)
			return nil
		},
		CreateRemote: func(_ context.Context, name, url string, auto bool, interval int) error {
			calls = append(calls, "create-remote "+url)
			if !auto || interval != 60 {
				t.Error("Apple's defaults: auto update every 60 minutes", auto, interval)
			}
			add(Profile{Name: name, Type: "remote", RemoteURL: url, AutoUpdate: auto, AutoUpdateInterval: interval, LastUpdated: time.Now()}, "{}")
			return nil
		},
		UpdateRemote: func(_ context.Context, id string) error {
			calls = append(calls, "update "+stored[index(id)].Name)
			return nil
		},
		SaveProfile: func(_ context.Context, id string, set ProfileSettings) error {
			stored[index(id)].Name = set.Name
			return nil
		},
		ReadContent:   func(_ context.Context, id string) (string, error) { return contents[id], nil },
		UpdateContent: func(_ context.Context, id string, json string) error { contents[id] = json; return nil },
		Select: func(_ context.Context, id string) error {
			for i := range stored {
				stored[i].Selected = stored[i].ID == id
			}
			return nil
		},
		Delete: func(_ context.Context, id string) error {
			stored = append(stored[:index(id)], stored[index(id)+1:]...)
			return nil
		},
		OpenFile:    func(context.Context) (string, []byte, error) { return openName, openData, nil },
		ProfileName: func(_ context.Context, data []byte) (string, error) { return string(data), nil },
		ImportProfile: func(_ context.Context, data []byte) error {
			add(Profile{Name: string(data), Type: "local"}, "{}")
			return nil
		},
		EncodeProfile: func(_ context.Context, id string) ([]byte, error) { return []byte("bpf:" + id), nil },
		SaveFile: func(_ context.Context, name string, data []byte) error {
			calls = append(calls, "save "+name+" "+string(data))
			return nil
		},
	}
	s := New(Config{Callbacks: cb})
	tt := ui.NewTester(s.Render, 1000, 760)
	complete(t, s, tt)
	click := func(labels ...string) {
		t.Helper()
		for _, label := range labels {
			if err := tt.Click(label); err != nil {
				t.Fatal(label, err, tt.Texts())
			}
		}
	}
	retype := func(field, text string) {
		t.Helper()
		click(field)
		tt.Key(ui.Ctrl, ui.KeyA)
		tt.Type(text)
	}
	click("其他", "管理配置")
	// Create Manually writes "{}", as Apple's NewProfileView does.
	click("新建配置", "手动创建", "名称")
	tt.Type("测试配置")
	click("创建")
	complete(t, s, tt)
	if len(stored) != 1 || contents["1"] != "{}" || !tt.HasText("测试配置") {
		t.Fatalf("stored=%v texts=%v", stored, tt.Texts())
	}
	// Edit Profile opens the content editor.
	click("配置操作 测试配置", "编辑", "编辑内容")
	complete(t, s, tt)
	retype("JSON", `{"log":{"level":"warn"}}`)
	click("保存")
	complete(t, s, tt)
	if contents["1"] != `{"log":{"level":"warn"}}` {
		t.Fatal(contents)
	}
	click("配置操作 测试配置", "编辑")
	retype("名称", "新名称")
	click("保存")
	complete(t, s, tt)
	if stored[0].Name != "新名称" {
		t.Fatal(stored)
	}
	// A remote profile: URL, then the picker's Update.
	click("新建配置", "手动创建", "名称")
	tt.Type("订阅")
	click("远程", "订阅 URL")
	tt.Type("https://example.com/sub.json")
	click("创建")
	complete(t, s, tt)
	if len(stored) != 2 || stored[1].Type != "remote" || !tt.HasText("远程 · 更新于 刚刚") {
		t.Fatal(stored, tt.Texts())
	}
	click("配置操作 订阅", "更新")
	complete(t, s, tt)
	// Selecting another profile.
	click("新名称")
	complete(t, s, tt)
	if !stored[0].Selected || stored[1].Selected {
		t.Fatal("selection not persisted", stored)
	}
	// Import from File: a .bpf asks first; a .json opens New Profile.
	openName, openData = `C:\x\shared.bpf`, []byte("导入项")
	click("新建配置", "从文件导入")
	complete(t, s, tt)
	if !tt.HasText("确定导入配置“导入项”？") {
		t.Fatal(tt.Texts())
	}
	click("导入")
	complete(t, s, tt)
	openName, openData = `C:\x\本地文件.json`, []byte(`{"dns":{}}`)
	click("新建配置", "从文件导入")
	complete(t, s, tt)
	click("创建")
	complete(t, s, tt)
	if len(stored) != 4 || stored[2].Name != "导入项" || stored[3].Name != "本地文件" || contents[stored[3].ID] != `{"dns":{}}` {
		t.Fatal(stored, contents)
	}
	click("配置操作 订阅", "删除", "删除")
	complete(t, s, tt)
	if len(stored) != 3 {
		t.Fatal(stored)
	}
	// The dashboard card shares the selected profile.
	click("其他", "概览", "分享配置", "保存文件")
	complete(t, s, tt)
	want := []string{"create-remote https://example.com/sub.json", "update 订阅", "save 本地文件.bpf bpf:4"}
	if strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Fatal(calls)
	}
}

func TestFailureKeepsEditorAndDoesNotInventProfile(t *testing.T) {
	s := New(Config{Callbacks: Callbacks{List: func(context.Context) ([]Profile, error) { return nil, nil }, Create: func(context.Context, string, string) error {
		return errors.New("official checker rejected configuration")
	}}})
	tt := ui.NewTester(s.Render, 1000, 760)
	complete(t, s, tt)
	for _, label := range []string{"新建配置", "手动创建", "名称"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	tt.Type("坏配置")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	complete(t, s, tt)
	if !s.dialog || len(s.profiles) != 0 || !tt.HasText("official checker rejected configuration") {
		t.Fatal(tt.Texts())
	}
}

func TestServiceActionsAreExplicitAndObserved(t *testing.T) {
	starts, stops, statusCalls := 0, 0, 0
	state := ServiceState{Available: true, Owned: true, Message: "真实未连接"}
	s := New(Config{Callbacks: Callbacks{
		List:   func(context.Context) ([]Profile, error) { return []Profile{{ID: "one", Name: "本地"}}, nil },
		Status: func(context.Context) (ServiceState, error) { statusCalls++; return state, nil },
		Start: func(context.Context) error {
			starts++
			state.Running = true
			state.Message = "真实已连接"
			return nil
		},
		Stop: func(context.Context) error {
			stops++
			state.Running = false
			state.Message = "真实未连接"
			return nil
		},
	}})
	tt := ui.NewTester(s.Render, 1000, 760)
	complete(t, s, tt)
	// Launch connects read-only, like the Apple client; it never starts or stops.
	if starts != 0 || stops != 0 || statusCalls != 1 || !s.service.Available {
		t.Fatal("launch must only read the service state", starts, stops, statusCalls)
	}
	if err := tt.Click("检测服务"); err != nil {
		t.Fatal(err)
	}
	complete(t, s, tt)
	if statusCalls != 2 {
		t.Fatal("manual check did not read the service state")
	}
	if err := tt.Click("启动"); err != nil {
		t.Fatal(err)
	}
	complete(t, s, tt)
	if starts != 1 || !s.service.Running || len(s.profiles) != 1 {
		t.Fatal("start result not observed")
	}
	if err := tt.Click("停止"); err != nil {
		t.Fatal(err)
	}
	complete(t, s, tt)
	if stops != 1 || s.service.Running {
		t.Fatal("stop result not observed")
	}
}

func TestSpecimenScreenshots(t *testing.T) {
	output := filepath.Join("..", "..", "..", "build", "ui-reference")
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"zh-Hans", "en"} {
		for _, dark := range []bool{false, true} {
			s := New(Config{Language: language})
			s.loaded = true
			s.service = ServiceState{Running: true, Metrics: &Metrics{Upload: "50.6 KB/s", Download: "2.1 MB/s", UploadTotal: "1.2 GB", DownloadTotal: "8.6 GB", Memory: "37 MB", Goroutines: "152", Inbound: "19", Outbound: "20"}}
			s.uploadHistory = []float32{4, 1, 0, 3, 2, 0, 9, 2, 6, 4, 3, 15, 50, 12, 7, 1, 2}
			s.downloadHistory = []float32{9, 2, 4, 3, 0, 2, 1, 3, 0, 5, 4, 2, 15, 55, 7, 22, 12}
			// Explicit test-only data; the production shell never creates these values.
			s.profiles = []Profile{{ID: "one", Name: "PNetwork", Type: "local", Selected: true}, {ID: "two", Name: "Work", Type: "local"}}
			s.config.Callbacks.WatchGroups = func(ctx context.Context, _ func([]Group)) error { <-ctx.Done(); return ctx.Err() }
			s.groupsReceived = true
			s.groups = []Group{
				{Name: "PROXY", Type: "Selector", Selectable: true, Expanded: true, Nodes: []Node{{Name: "Auto Select", Type: "URLTest", DelayMS: 163}, {Name: "Tokyo", Type: "VLESS", DelayMS: 890, Selected: true}, {Name: "Singapore", Type: "Hysteria2", DelayMS: 2055}, {Name: "Hong Kong", Type: "VMess"}, {Name: "Los Angeles", Type: "VLESS"}, {Name: "direct", Type: "Direct"}}},
				{Name: "Auto", Type: "URLTest", Nodes: []Node{{Name: "a", DelayMS: 120, Selected: true}, {Name: "b", DelayMS: 950}, {Name: "c", DelayMS: 1700}, {Name: "d"}}},
			}
			s.logs.apply(LogFrame{Reset: true, DefaultLevel: 4, Lines: []LogLine{
				{4, "[36mINFO[0m network: updated default interface Ethernet, index 12"},
				{4, "[36mINFO[0m inbound/tun[tun-in]: started at ProxyGlassTest"},
				{4, "[36mINFO[0m inbound/mixed[mixed-in]: tcp server started at 127.0.0.1:20808"},
				{3, "[33mWARN[0m outbound/urltest[auto]: outbound direct-c unavailable: context deadline exceeded"},
				{2, "[31mERROR[0m [1754212345 0ms] inbound/mixed[mixed-in]: process connection from 127.0.0.1:52011: EOF"},
				{4, "[36mINFO[0m sing-box started (0.31s)"},
			}})
			now := time.Now().UnixMilli()
			specimen := func(id, network, dest, domain string, created int64, up, down int64) *Connection {
				return &Connection{ID: id, Network: network, Destination: dest, Domain: domain, Inbound: "mixed-in", InboundType: "mixed", Outbound: "Tokyo", OutboundType: "vless", Chain: []string{"Tokyo", "PROXY"}, CreatedAt: created, UplinkTotal: up * 40, DownlinkTotal: down * 40}
			}
			s.conns.table.apply(ConnectionEvents{Reset: true, Events: []ConnectionEvent{
				{Type: ConnectionNew, ID: "1", Connection: specimen("1", "tcp", "142.250.70.36:443", "www.example.com", now-4000, 2100, 98000)},
				{Type: ConnectionNew, ID: "2", Connection: specimen("2", "udp", "1.1.1.1:443", "", now-3000, 300, 1200)},
				{Type: ConnectionNew, ID: "3", Connection: specimen("3", "tcp", "93.184.216.34:80", "example.org", now-2000, 800, 5100)},
				{Type: ConnectionUpdate, ID: "1", UplinkDelta: 2100, DownlinkDelta: 98000},
			}}, now)
			tt := ui.NewTester(s.Render, 1000, 760)
			tt.SetDark(dark)
			mode := "light"
			if dark {
				mode = "dark"
			}
			save := func(name string) {
				t.Helper()
				f, err := os.Create(filepath.Join(output, language+"-"+mode+"-"+name+".png"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
			}
			save("dashboard")
			for _, page := range []string{"groups", "connections", "logs", "profiles", "settings"} {
				s.page = page
				tt.Frame()
				save(page)
			}
			if err := tt.Click(s.text("其他", "More")); err != nil {
				t.Fatal(err)
			}
			save("menu")
		}
	}
}

func TestGroupsEmptyAndMetricsUnavailable(t *testing.T) {
	s := New(Config{})
	s.loaded = true
	s.service.Running = true
	tt := ui.NewTester(s.Render, 1000, 760)
	if !tt.HasText("—") {
		t.Fatal("unavailable metrics should use em dash")
	}
	if err := tt.Click("分组"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("暂无可用分组") {
		t.Fatal(tt.Texts())
	}
}

func TestLocaleFontFamily(t *testing.T) {
	if got := New(Config{Language: "zh-Hans"}).fontFamily(); got != "Microsoft YaHei UI" {
		t.Fatal(got)
	}
	if got := New(Config{Language: "en"}).fontFamily(); got != "Segoe UI" {
		t.Fatal(got)
	}
}

func TestGroupsFollowAppleInteractions(t *testing.T) {
	var calls []string
	canceled := make(chan struct{})
	s := New(Config{Callbacks: Callbacks{
		WatchGroups: func(ctx context.Context, emit func([]Group)) error {
			emit([]Group{
				{Name: "Work", Type: "Selector", Selectable: true, Expanded: true, Nodes: []Node{{Name: "Tokyo", Type: "VLESS", Selected: true, DelayMS: 890}, {Name: "Singapore", Type: "Hysteria2"}}},
				{Name: "Travel", Type: "URLTest", Nodes: []Node{{Name: "Osaka", Type: "VMess", Selected: true}}},
			})
			<-ctx.Done()
			close(canceled)
			return ctx.Err()
		},
		SelectOutbound: func(_ context.Context, g, o string) error { calls = append(calls, "select "+g+"/"+o); return nil },
		URLTest:        func(_ context.Context, tag string) error { calls = append(calls, "test "+tag); return nil },
		SetGroupExpand: func(_ context.Context, g string, e bool) error {
			calls = append(calls, fmt.Sprintf("expand %s %v", g, e))
			return nil
		},
	}})
	s.loaded, s.page, s.service.Running = true, "groups", true
	tt := ui.NewTester(s.Render, 1000, 760)
	for deadline := time.Now().Add(time.Second); !tt.HasText("Tokyo"); tt.Frame() {
		if time.Now().After(deadline) {
			t.Fatal("groups never arrived", tt.Texts())
		}
	}
	if !tt.HasText("890ms") || tt.HasText("Osaka") {
		t.Fatal("expanded group must list nodes, collapsed group only dots", tt.Texts())
	}
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		complete(t, s, tt)
	}
	click("Tokyo") // already selected: no request
	click("Singapore")
	click("测速 Work")
	click("展开 Travel")
	if !s.groups[1].Expanded || !tt.HasText("Osaka") {
		t.Fatal("expansion is not applied optimistically", tt.Texts())
	}
	click("Osaka") // URLTest groups are not selectable
	want := []string{"select Work/Singapore", "test Work", "expand Travel true"}
	if fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
	if err := tt.Click("概览"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("leaving the page did not cancel the groups subscription")
	}
}

func TestServiceMessageFollowsLanguage(t *testing.T) {
	for language, want := range map[string]string{"en": "Service available, core not started", "zh-Hans": "服务可用，核心未启动"} {
		s := New(Config{Language: language})
		s.service.Message = "core_idle"
		if got := s.serviceMessage(); got != want {
			t.Errorf("%s: serviceMessage = %q, want %q", language, got, want)
		}
	}
	s := New(Config{Language: "en"})
	s.service.Message = "free text"
	if got := s.serviceMessage(); got != "free text" {
		t.Errorf("free text = %q", got)
	}
}
