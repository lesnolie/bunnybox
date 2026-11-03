package shell

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Profile mirrors the Apple client's profile preview. Type is "local" or
// "remote"; the remote fields are empty for local profiles.
type Profile struct {
	ID, Name, Type     string
	Selected           bool
	RemoteURL          string
	AutoUpdate         bool
	AutoUpdateInterval int // minutes; 0 is the one hour default
	LastUpdated        time.Time
}

// ProfileSettings are the fields of Apple's Edit Profile form.
type ProfileSettings struct {
	Name, RemoteURL    string
	AutoUpdate         bool
	AutoUpdateInterval int
}

var errUnavailable = errors.New("本地配置后端或官方配置检查不可用 / Profile backend or official configuration checker unavailable")

// profileForm is the open profile dialog: Apple's New Profile, Edit Profile
// and content editor, a delete confirmation, or a .bpf import confirmation.
type profileForm struct {
	action      string // new, edit, content, delete, import
	id, name    string
	remote      int // New Profile's type: 0 local, 1 remote
	fileImport  int // 0 create new, 1 import a chosen file
	fileName    string
	content     string
	url         string
	autoUpdate  bool
	interval    string
	readOnly    bool
	lastUpdated time.Time
	importData  []byte
	saved       ProfileSettings // Edit Profile's state when opened
	editingKind string          // the edited profile's type
}

// openedFile is a profile file the host's open dialog read.
type openedFile struct {
	name     string // file name without extension
	json     bool
	data     []byte
	bpfName  string // decoded profile name of a .bpf file
	canceled bool
}

func (s *Shell) selectedProfile() *Profile {
	for i := range s.profiles {
		if s.profiles[i].Selected {
			return &s.profiles[i]
		}
	}
	return nil
}

func (s *Shell) profileType(p Profile) string {
	if p.Type == "remote" {
		return s.text("远程", "Remote")
	}
	return s.text("本地", "Local")
}

// relativeTime follows Apple's relativeFormat.
func (s *Shell) relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return s.text("刚刚", "just now")
	case d < time.Hour:
		n := int(d / time.Minute)
		return s.text(strconv.Itoa(n)+" 分钟前", strconv.Itoa(n)+" min. ago")
	case d < 24*time.Hour:
		n := int(d / time.Hour)
		return s.text(strconv.Itoa(n)+" 小时前", strconv.Itoa(n)+" hr. ago")
	default:
		n := int(d / (24 * time.Hour))
		return s.text(strconv.Itoa(n)+" 天前", strconv.Itoa(n)+" days ago")
	}
}

// newProfileMenu is Apple's NewProfileMenuView: create manually or import a
// file. QR imports have no Windows counterpart yet.
func (s *Shell) newProfileMenu(c *ui.Context, m *ui.Menu) {
	if m.Item(s.text("手动创建", "Create Manually")).Disabled(s.busy).Chosen() {
		s.openNewProfile(openedFile{})
	}
	if m.Item(s.text("从文件导入", "Import from File")).Disabled(s.busy || s.config.Callbacks.OpenFile == nil).Chosen() {
		s.importFromFile(c)
	}
}

// dashboardProfile is Apple's ProfileCard.
func (s *Shell) dashboardProfile(c *ui.Context, t *ui.Theme) {
	selected := s.selectedProfile()
	s.card(c, t, func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["logs"]).FontSize(20)
			ui.Text(c, s.text("配置", "Profile")).FontSize(17).FontWeight(600).Grow(1)
			dashboardAction(c, t, icons["add"], s.text("新建配置", "New profile")).Disabled(s.busy).PopoverMenuButton(func(m *ui.Menu) { s.newProfileMenu(c, m) })
		})
		if len(s.profiles) == 0 {
			ui.Text(c, s.text("没有配置", "Empty profiles")).Padding(8, 0).TextColor(t.TextMuted)
			return
		}
		label := s.text("选择配置", "Select profile")
		if selected != nil {
			label = selected.Name
		}
		ui.Button(c, "").Label(s.text("选择配置", "Select profile")).Height(40).Padding(10, 14).Justify(ui.Start).Background(t.Background).Border(0, ui.Transparent).Radius(10).Children(func() {
			ui.Text(c, label).FontSize(16).FontWeight(600).Grow(1)
			ui.Icon(c, icons["chevrons"]).FontSize(15).TextColor(t.TextMuted)
		}).PopoverMenuButton(func(menu *ui.Menu) {
			for _, p := range s.profiles {
				if menu.Item(p.Name).Radio(p.Selected).Disabled(s.busy).Chosen() && !p.Selected {
					s.selectProfile(c, p.ID)
				}
			}
		})
		if selected == nil {
			return
		}
		p := *selected
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.profileType(p)).FontSize(12)
			if p.Type == "remote" && !p.LastUpdated.IsZero() {
				ui.Text(c, "·").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, s.text("更新于 ", "Updated ")+s.relativeTime(p.LastUpdated)).FontSize(12)
			}
		})
		ui.Row(c).Gap(12).Children(func() {
			if dashboardAction(c, t, icons["edit"], s.text("编辑配置", "Edit profile")).Disabled(s.busy).Clicked() {
				s.openEdit(p)
			}
			if p.Type == "remote" && dashboardAction(c, t, icons["refresh"], s.text("更新配置", "Update profile")).Disabled(s.busy).Clicked() {
				s.updateRemote(c, p.ID)
			}
			dashboardAction(c, t, icons["share"], s.text("分享配置", "Share profile")).Disabled(s.busy).PopoverMenuButton(func(m *ui.Menu) { s.shareMenu(c, m, p) })
		})
	})
}

// profileSection is the profile list (Apple's ProfilePickerSheet).
func (s *Shell) profileSection(c *ui.Context, t *ui.Theme) {
	s.card(c, t, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["profiles"]).FontSize(18)
			ui.Text(c, s.text("配置", "Profiles")).FontSize(16).Bold().Grow(1)
			iconButton(c, icons["add"], s.text("新建配置", "New profile")).Disabled(s.busy).PopoverMenuButton(func(m *ui.Menu) { s.newProfileMenu(c, m) })
			if iconButton(c, icons["refresh"], s.text("刷新配置", "Refresh profiles")).Disabled(s.busy).Clicked() {
				s.reload(c)
			}
		})
		if s.busy {
			ui.Text(c, s.text("处理中…", "Working…")).TextColor(t.TextMuted)
		}
		if len(s.profiles) == 0 {
			ui.Column(c).Padding(16, 0).Gap(8).Children(func() {
				ui.Text(c, s.text("没有配置", "Empty profiles")).FontSize(15)
				ui.Text(c, s.text("添加配置以开始使用。", "Add a profile to get started.")).FontSize(12).TextColor(t.TextMuted)
			})
		}
		for _, p := range s.profiles {
			ui.Row(c).Key(p.ID).Gap(10).AlignItems(ui.Center).Padding(6).Children(func() {
				if ui.Button(c, "").Label(p.Name).Grow(1).Height(44).Background(ui.Transparent).Border(0, ui.Transparent).Children(func() {
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, p.Name).FontSize(14)
						info := s.profileType(p)
						if p.Type == "remote" && !p.LastUpdated.IsZero() {
							info += " · " + s.text("更新于 ", "Updated ") + s.relativeTime(p.LastUpdated)
						}
						ui.Text(c, info).FontSize(11).TextColor(t.TextMuted)
					})
					ico := icons["circle"]
					if p.Selected {
						ico = icons["selected"]
					}
					ui.Icon(c, ico).FontSize(18).TextColor(t.Accent)
				}).Disabled(s.busy).Clicked() && !p.Selected {
					s.selectProfile(c, p.ID)
				}
				ui.Button(c, "").Label(s.text("配置操作 ", "Profile actions ")+p.Name).Height(28).Width(28).Background(ui.Transparent).Border(0, ui.Transparent).Children(func() { ui.Icon(c, icons["more"]).FontSize(16) }).PopoverMenuButton(func(m *ui.Menu) { s.profileMenu(c, m, p) })
			})
		}
	})
}

// profileMenu is a picker row's menu: Edit, Update, Share and Delete.
func (s *Shell) profileMenu(c *ui.Context, m *ui.Menu, p Profile) {
	if m.Item(s.text("编辑", "Edit")).Disabled(s.busy).Chosen() {
		s.openEdit(p)
	}
	if p.Type == "remote" && m.Item(s.text("更新", "Update")).Disabled(s.busy).Chosen() {
		s.updateRemote(c, p.ID)
	}
	m.Submenu(s.text("分享", "Share"), func(m *ui.Menu) { s.shareMenu(c, m, p) })
	m.Separator()
	if m.Item(s.text("删除", "Delete")).Disabled(s.busy).Chosen() {
		s.form = profileForm{action: "delete", id: p.ID, name: p.Name}
		s.dialog, s.errorText = true, ""
	}
}

// shareMenu is Apple's share menu, without the macOS share sheet and QR codes.
func (s *Shell) shareMenu(c *ui.Context, m *ui.Menu, p Profile) {
	cb := s.config.Callbacks
	if m.Item(s.text("保存文件", "Save File")).Disabled(s.busy || cb.EncodeProfile == nil || cb.SaveFile == nil).Chosen() {
		s.groupAction(c, func(ctx context.Context) error {
			data, err := cb.EncodeProfile(ctx, p.ID)
			if err != nil {
				return err
			}
			return cb.SaveFile(ctx, p.Name+".bpf", data)
		})
	}
	if m.Item(s.text("保存内容 JSON", "Save Content JSON")).Disabled(s.busy || cb.ReadContent == nil || cb.SaveFile == nil).Chosen() {
		s.groupAction(c, func(ctx context.Context) error {
			content, err := cb.ReadContent(ctx, p.ID)
			if err != nil {
				return err
			}
			return cb.SaveFile(ctx, p.Name+".json", []byte(content))
		})
	}
}

func (s *Shell) openNewProfile(f openedFile) {
	s.form = profileForm{action: "new", autoUpdate: true, interval: "60"}
	if f.json {
		s.form.name, s.form.fileImport, s.form.fileName, s.form.content = f.name, 1, f.name+".json", string(f.data)
	}
	s.dialog, s.errorText = true, ""
}

func (s *Shell) openEdit(p Profile) {
	s.form = profileForm{action: "edit", id: p.ID, name: p.Name, url: p.RemoteURL, autoUpdate: p.AutoUpdate, interval: strconv.Itoa(p.AutoUpdateInterval), lastUpdated: p.LastUpdated, editingKind: p.Type}
	if p.AutoUpdateInterval == 0 {
		s.form.interval = "60"
	}
	s.form.saved = s.form.settings()
	s.dialog, s.errorText = true, ""
}

func (f profileForm) settings() ProfileSettings {
	return ProfileSettings{Name: strings.TrimSpace(f.name), RemoteURL: strings.TrimSpace(f.url), AutoUpdate: f.autoUpdate, AutoUpdateInterval: f.minutes()}
}

// minutes parses the interval as Apple's stringBinding(defaultValue: 60) does.
func (f profileForm) minutes() int {
	n, err := strconv.Atoi(strings.TrimSpace(f.interval))
	if err != nil || n < 0 {
		return 60
	}
	return n
}

// importFromFile is Apple's file import: JSON opens New Profile, a .bpf file
// asks to import the profile it holds.
func (s *Shell) importFromFile(c *ui.Context) {
	cb := s.config.Callbacks
	s.run(c, func(ctx context.Context) result {
		name, data, err := cb.OpenFile(ctx)
		if err != nil || name == "" {
			return result{err: err, opened: &openedFile{canceled: true}}
		}
		base := filepath.Base(name)
		f := openedFile{name: strings.TrimSuffix(base, filepath.Ext(base)), data: data}
		if strings.EqualFold(filepath.Ext(name), ".json") {
			f.json = true
			return result{opened: &f}
		}
		if cb.ProfileName == nil {
			return result{err: errUnavailable}
		}
		// The user may browse the dialog for longer than an operation may
		// take: decoding gets its own deadline.
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		f.bpfName, err = cb.ProfileName(ctx, data)
		return result{opened: &f, err: err}
	})
}

func (s *Shell) applyOpened(f openedFile) {
	switch {
	case f.canceled:
	case f.json:
		s.openNewProfile(f)
	default:
		s.form = profileForm{action: "import", name: f.bpfName, importData: f.data}
		s.dialog, s.errorText = true, ""
	}
}

// readContent opens the content editor, read-only for remote profiles as
// Apple's View Content.
func (s *Shell) readContent(c *ui.Context, id, name string, readOnly bool) {
	read := s.config.Callbacks.ReadContent
	s.pendingForm = profileForm{action: "content", id: id, name: name, readOnly: readOnly}
	s.run(c, func(ctx context.Context) result {
		if read == nil {
			return result{err: errUnavailable}
		}
		content, err := read(ctx, id)
		return result{content: content, loadContent: true, err: err}
	})
}

func (s *Shell) updateRemote(c *ui.Context, id string) {
	update := s.config.Callbacks.UpdateRemote
	s.mutateProfiles(c, func(ctx context.Context) error {
		if update == nil {
			return errUnavailable
		}
		return update(ctx, id)
	})
}

// mutateProfiles runs a profile change, then lists the profiles again.
func (s *Shell) mutateProfiles(c *ui.Context, op func(context.Context) error) {
	s.run(c, func(ctx context.Context) result {
		if err := op(ctx); err != nil {
			return result{err: err}
		}
		p, err := s.list(ctx)
		return result{profiles: p, err: err}
	})
}

func (s *Shell) editor(c *ui.Context, t *ui.Theme) {
	f := &s.form
	ui.Modal(c, &s.dialog, func() {
		width := float32(430)
		if f.action == "content" {
			width = 640
		}
		ui.Column(c).Width(width).Gap(12).Children(func() {
			switch f.action {
			case "new":
				s.newProfileForm(c, t)
			case "edit":
				s.editProfileForm(c, t)
			case "content":
				title := s.text("编辑内容", "Edit Content")
				if f.readOnly {
					title = s.text("查看内容", "View Content")
				}
				ui.Text(c, title+" · "+f.name).FontSize(17).Bold()
				ui.TextArea(c, &f.content).Label("JSON").Height(360).Font("Consolas").Disabled(s.busy || f.readOnly)
			case "delete":
				ui.Text(c, s.text("删除配置？", "Delete profile?")).FontSize(17).Bold()
				ui.Text(c, f.name)
			case "import":
				ui.Text(c, s.text("导入配置", "Import Profile")).FontSize(17).Bold()
				ui.Text(c, s.text("确定导入配置“"+f.name+"”？", "Are you sure to import profile "+f.name+"?"))
			}
			if s.errorText != "" {
				ui.Text(c, s.errorText).TextColor(t.Danger).FontSize(11)
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if f.action == "edit" {
					label, readOnly := s.text("编辑内容", "Edit Content"), f.editingKind == "remote"
					if readOnly {
						label = s.text("查看内容", "View Content")
					}
					if ui.Button(c, label).Disabled(s.busy).Clicked() {
						s.readContent(c, f.id, f.name, readOnly)
					}
				}
				ui.Column(c).Grow(1)
				closeLabel := s.text("取消", "Cancel")
				if f.action == "content" && f.readOnly {
					closeLabel = s.text("关闭", "Close")
				}
				if ui.Button(c, closeLabel).Disabled(s.busy).Clicked() {
					s.dialog = false
				}
				if f.action == "content" && f.readOnly {
					return
				}
				confirm := map[string][2]string{"new": {"创建", "Create"}, "edit": {"保存", "Save"}, "content": {"保存", "Save"}, "delete": {"删除", "Delete"}, "import": {"导入", "Import"}}[f.action]
				disabled := s.busy || f.action == "edit" && f.settings() == f.saved
				if ui.PrimaryButton(c, s.text(confirm[0], confirm[1])).Disabled(disabled).Clicked() {
					s.save(c)
				}
			})
		})
	})
}

// formRow lays out one row of Apple's FormView: a label and its control.
func formRow(c *ui.Context, t *ui.Theme, label string, control func()) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).MinHeight(34).Children(func() {
		ui.Text(c, label).Width(120).TextColor(t.Text)
		ui.Row(c).Grow(1).Justify(ui.End).Children(control)
	})
}

// newProfileForm is Apple's NewProfileView.
func (s *Shell) newProfileForm(c *ui.Context, t *ui.Theme) {
	f := &s.form
	ui.Text(c, s.text("新建配置", "New Profile")).FontSize(17).Bold()
	formRow(c, t, s.text("名称", "Name"), func() {
		ui.TextInput(c, &f.name).Label(s.text("名称", "Name")).Placeholder(s.text("必填", "Required")).Grow(1).Disabled(s.busy)
	})
	formRow(c, t, s.text("类型", "Type"), func() {
		ui.Segmented(c, &f.remote, s.text("本地", "Local"), s.text("远程", "Remote")).Label(s.text("类型", "Type")).Disabled(s.busy)
	})
	if f.remote == 0 {
		formRow(c, t, s.text("文件", "File"), func() {
			ui.Segmented(c, &f.fileImport, s.text("新建", "Create New"), s.text("导入", "Import")).Label(s.text("文件", "File")).Disabled(s.busy)
		})
		if f.fileImport == 1 {
			formRow(c, t, s.text("文件路径", "File Path"), func() {
				label := s.text("选择", "Choose")
				if f.fileName != "" {
					label = f.fileName
				}
				if ui.Button(c, label).Label(s.text("选择文件", "Choose file")).Disabled(s.busy || s.config.Callbacks.OpenFile == nil).Clicked() {
					s.chooseJSON(c)
				}
			})
		}
		return
	}
	s.remoteFields(c, t)
}

// remoteFields are the URL and auto update rows of Apple's profile forms.
func (s *Shell) remoteFields(c *ui.Context, t *ui.Theme) {
	f := &s.form
	formRow(c, t, "URL", func() {
		ui.TextInput(c, &f.url).Label(s.text("订阅 URL", "URL")).Placeholder(s.text("必填", "Required")).Grow(1).Disabled(s.busy)
	})
	formRow(c, t, s.text("自动更新", "Auto Update"), func() {
		ui.Switch(c, &f.autoUpdate).Label(s.text("自动更新", "Auto Update")).Disabled(s.busy)
	})
	formRow(c, t, s.text("自动更新间隔", "Auto Update Interval"), func() {
		ui.TextInput(c, &f.interval).Label(s.text("自动更新间隔", "Auto Update Interval")).Placeholder(s.text("分钟", "In Minutes")).Width(120).Disabled(s.busy)
		ui.Text(c, s.text("分钟", "min")).TextColor(t.TextMuted)
	})
}

// editProfileForm is Apple's EditProfileView.
func (s *Shell) editProfileForm(c *ui.Context, t *ui.Theme) {
	f := &s.form
	ui.Text(c, s.text("编辑配置", "Edit Profile")).FontSize(17).Bold()
	formRow(c, t, s.text("名称", "Name"), func() {
		ui.TextInput(c, &f.name).Label(s.text("名称", "Name")).Placeholder(s.text("必填", "Required")).Grow(1).Disabled(s.busy)
	})
	formRow(c, t, s.text("类型", "Type"), func() {
		ui.Text(c, s.profileType(Profile{Type: f.editingKind})).TextColor(t.TextMuted)
	})
	if f.editingKind != "remote" {
		return
	}
	s.remoteFields(c, t)
	ui.Text(c, s.text("状态", "Status")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
	formRow(c, t, s.text("上次更新", "Last Updated"), func() {
		text := "—"
		if !f.lastUpdated.IsZero() {
			text = s.relativeTime(f.lastUpdated)
		}
		ui.Text(c, text).TextColor(t.TextMuted)
	})
}

// chooseJSON picks the file of New Profile's local import.
func (s *Shell) chooseJSON(c *ui.Context) {
	open := s.config.Callbacks.OpenFile
	s.run(c, func(ctx context.Context) result {
		name, data, err := open(ctx)
		if err != nil || name == "" {
			return result{err: err, ack: true}
		}
		if !strings.EqualFold(filepath.Ext(name), ".json") {
			return result{err: errors.New("请选择 JSON 文件 / choose a JSON file"), ack: true}
		}
		return result{chosen: &openedFile{name: name, json: true, data: data}, ack: true}
	})
}

func (s *Shell) save(c *ui.Context) {
	f := s.form
	cb := s.config.Callbacks
	missing := func(text string) { s.errorText = text }
	var op func(context.Context) error
	switch f.action {
	case "new":
		name := strings.TrimSpace(f.name)
		switch {
		case name == "":
			missing(s.text("缺少配置名称", "Missing profile name"))
			return
		case f.remote == 1 && strings.TrimSpace(f.url) == "":
			missing(s.text("缺少 URL", "Missing URL"))
			return
		case f.remote == 0 && f.fileImport == 1 && f.fileName == "":
			missing(s.text("缺少文件", "Missing file"))
			return
		}
		if f.remote == 1 {
			if cb.CreateRemote != nil {
				url, auto, minutes := strings.TrimSpace(f.url), f.autoUpdate, f.minutes()
				op = func(ctx context.Context) error { return cb.CreateRemote(ctx, name, url, auto, minutes) }
			}
		} else if cb.Create != nil {
			content := "{}"
			if f.fileImport == 1 {
				content = f.content
			}
			op = func(ctx context.Context) error { return cb.Create(ctx, name, content) }
		}
	case "edit":
		if strings.TrimSpace(f.name) == "" {
			missing(s.text("缺少配置名称", "Missing profile name"))
			return
		}
		if f.editingKind == "remote" && strings.TrimSpace(f.url) == "" {
			missing(s.text("缺少 URL", "Missing URL"))
			return
		}
		if cb.SaveProfile != nil {
			id, settings := f.id, f.settings()
			op = func(ctx context.Context) error { return cb.SaveProfile(ctx, id, settings) }
		}
	case "content":
		if cb.UpdateContent != nil {
			id, content := f.id, f.content
			op = func(ctx context.Context) error { return cb.UpdateContent(ctx, id, content) }
		}
	case "delete":
		if cb.Delete != nil {
			id := f.id
			op = func(ctx context.Context) error { return cb.Delete(ctx, id) }
		}
	case "import":
		if cb.ImportProfile != nil {
			data := f.importData
			op = func(ctx context.Context) error { return cb.ImportProfile(ctx, data) }
		}
	}
	s.mutateProfiles(c, func(ctx context.Context) error {
		if op == nil {
			return errUnavailable
		}
		return op(ctx)
	})
}
