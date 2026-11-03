package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"singboxwin.local/client/internal/daemon"
	"singboxwin.local/client/internal/profile"
	"singboxwin.local/client/internal/rpc/gen/desktop"
	"singboxwin.local/client/internal/ui/shell"
)

// Apple's ProfileUpdateTask: auto updates never run more often than every 15
// minutes; an interval of 0 means one hour.
const (
	minUpdateInterval     = 15 * time.Minute
	defaultUpdateInterval = time.Hour
	maxRemoteProfileSize  = 64 << 20
	fetchTimeout          = 30 * time.Second
)

func intervalOrDefault(minutes int64) time.Duration {
	if minutes > 0 {
		return time.Duration(minutes) * time.Minute
	}
	return defaultUpdateInterval
}

func uiProfile(p profile.Profile, selected bool) shell.Profile {
	out := shell.Profile{ID: strconv.FormatInt(p.ID, 10), Name: p.Name, Type: "local", Selected: selected}
	if p.Type == profile.TypeRemote {
		out.Type, out.RemoteURL, out.AutoUpdate, out.AutoUpdateInterval, out.LastUpdated = "remote", p.RemoteURL, p.AutoUpdate, int(p.AutoUpdateInterval), p.LastUpdated
	}
	return out
}

// userAgent follows Apple's HTTPClient: "<app> (sing-box <version>; language <locale>)".
// Subscription servers use the sing-box marker to choose the format.
func (b *Backend) userAgent(ctx context.Context) string {
	version, err := daemon.BinaryVersion(ctx, b.executable)
	if err != nil || version == "" {
		version = "unknown"
	}
	return "SFW (sing-box " + version + "; language zh_CN)"
}

// fetch downloads a remote profile like Apple's HTTPClient.getString.
func (b *Backend) fetch(ctx context.Context, address string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(address))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("订阅 URL 必须是 http 或 https 地址 / URL must be http or https")
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", b.userAgent(ctx))
	response, err := downloadHTTP.Do(request)
	if err != nil {
		return "", fmt.Errorf("下载订阅失败 / fetch: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载订阅失败 / fetch: HTTP %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxRemoteProfileSize+1))
	if err != nil {
		return "", fmt.Errorf("下载订阅失败 / fetch: %w", err)
	}
	if len(body) > maxRemoteProfileSize {
		return "", errors.New("订阅内容过大 / remote profile too large")
	}
	return string(body), nil
}

func (b *Backend) createRemote(ctx context.Context, name, address string, autoUpdate bool, interval int) error {
	if strings.TrimSpace(address) == "" {
		return errors.New("缺少订阅 URL / missing URL")
	}
	content, err := b.fetch(ctx, address)
	if err != nil {
		return err
	}
	_, err = b.profiles.Add(ctx, profile.New{Name: name, Type: profile.TypeRemote, Content: content, RemoteURL: strings.TrimSpace(address),
		AutoUpdate: autoUpdate, AutoUpdateInterval: int64(interval), LastUpdated: time.Now()})
	if err == nil {
		b.wakeUpdater()
	}
	return err
}

// updateRemote is Apple's updateRemoteProfile plus onProfileUpdated: a changed
// selected profile reloads a running core.
func (b *Backend) updateRemote(ctx context.Context, n int64) error {
	p, err := b.find(ctx, n)
	if err != nil {
		return err
	}
	if p.Type != profile.TypeRemote {
		return errors.New("不是远程配置 / not a remote profile")
	}
	content, err := b.fetch(ctx, p.RemoteURL)
	if err != nil {
		return err
	}
	changed, err := b.profiles.UpdateRemote(ctx, n, content, time.Now())
	if err != nil || !changed {
		return err
	}
	return b.reloadIfSelected(ctx, n)
}

func (b *Backend) find(ctx context.Context, n int64) (profile.Profile, error) {
	list, err := b.profiles.List(ctx)
	if err != nil {
		return profile.Profile{}, err
	}
	for _, p := range list {
		if p.ID == n {
			return p, nil
		}
	}
	return profile.Profile{}, profile.ErrNotFound
}

func (b *Backend) saveProfile(ctx context.Context, n int64, s shell.ProfileSettings) error {
	_, err := b.profiles.Save(ctx, n, profile.Settings{Name: s.Name, RemoteURL: strings.TrimSpace(s.RemoteURL), AutoUpdate: s.AutoUpdate, AutoUpdateInterval: int64(s.AutoUpdateInterval)})
	if err == nil {
		b.wakeUpdater()
	}
	return err
}

func (b *Backend) reloadIfSelected(ctx context.Context, n int64) error {
	selected, err := b.profiles.Selected(ctx)
	if err != nil || selected == nil || selected.ID != n {
		return err
	}
	return b.reload(ctx)
}

// reload restarts a core this user runs with the selected profile, as Apple's
// reloadService does after switching or changing it. The official desktop
// StartService reloads a started service in place.
func (b *Backend) reload(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, err := b.runtimeReader(ctx)
	if err != nil {
		return err
	}
	a, err := r.probe(ctx)
	if err != nil || !a.Available {
		return err // nothing of ours runs
	}
	p, err := b.profiles.Selected(ctx)
	if err != nil || p == nil {
		return err
	}
	content, err := b.profiles.ReadContent(ctx, p.ID)
	if err != nil {
		return err
	}
	if err = b.check(ctx, content); err != nil {
		return err
	}
	_, err = desktop.NewDesktopServiceClient(r.conn).StartService(ctx, &desktop.StartServiceRequest{ConfigContent: content, Options: &desktop.StartOptions{OomKillerDisabled: true}})
	return err
}

// encodeProfile is Apple's "Save File": the official binary profile (.bpf).
func (b *Backend) encodeProfile(ctx context.Context, n int64) ([]byte, error) {
	p, err := b.find(ctx, n)
	if err != nil {
		return nil, err
	}
	content, err := b.profiles.ReadContent(ctx, n)
	if err != nil {
		return nil, err
	}
	message := &desktop.ProfileContent{Name: p.Name, Type: desktop.ProfileContent_LOCAL, Config: content}
	if p.Type == profile.TypeRemote {
		message.Type, message.RemotePath, message.AutoUpdate, message.AutoUpdateInterval = desktop.ProfileContent_REMOTE, p.RemoteURL, p.AutoUpdate, int32(p.AutoUpdateInterval)
		if !p.LastUpdated.IsZero() {
			message.LastUpdated = p.LastUpdated.UnixMilli()
		}
	}
	conn, err := b.rpc.Application(ctx)
	if err != nil {
		return nil, err
	}
	data, err := desktop.NewApplicationServiceClient(conn).EncodeProfile(ctx, message)
	if err != nil {
		return nil, err
	}
	return data.GetData(), nil
}

func (b *Backend) decode(ctx context.Context, data []byte) (*desktop.ProfileContent, error) {
	conn, err := b.rpc.Application(ctx)
	if err != nil {
		return nil, err
	}
	return desktop.NewApplicationServiceClient(conn).DecodeProfile(ctx, &desktop.ProfileData{Data: data})
}

func (b *Backend) profileName(ctx context.Context, data []byte) (string, error) {
	content, err := b.decode(ctx, data)
	if err != nil {
		return "", err
	}
	return content.GetName(), nil
}

// importProfile is Apple's LibboxProfileContent.importProfile; iCloud
// profiles become local ones.
func (b *Backend) importProfile(ctx context.Context, data []byte) error {
	c, err := b.decode(ctx, data)
	if err != nil {
		return err
	}
	n := profile.New{Name: c.GetName(), Content: c.GetConfig()}
	if c.GetType() == desktop.ProfileContent_REMOTE {
		n.Type, n.RemoteURL, n.AutoUpdate, n.AutoUpdateInterval = profile.TypeRemote, c.GetRemotePath(), c.GetAutoUpdate(), int64(c.GetAutoUpdateInterval())
		if c.GetLastUpdated() > 0 {
			n.LastUpdated = time.UnixMilli(c.GetLastUpdated())
		}
	}
	if _, err = b.profiles.Add(ctx, n); err != nil {
		return err
	}
	b.wakeUpdater()
	return b.reload(ctx)
}

// StartAutoUpdate runs Apple's ProfileUpdateTask until Close; notify tells the
// UI that profiles changed.
func (b *Backend) StartAutoUpdate(notify func()) {
	ctx, cancel := context.WithCancel(context.Background())
	b.stopUpdater = cancel
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.wake:
			case <-timer.C:
			}
			next := b.autoUpdate(ctx, notify)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(next)
		}
	}()
}

func (b *Backend) wakeUpdater() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// autoUpdate updates due profiles and returns the wait until the next one.
func (b *Backend) autoUpdate(ctx context.Context, notify func()) time.Duration {
	list, err := b.profiles.List(ctx)
	if err != nil {
		return minUpdateInterval
	}
	next, now, updated := time.Duration(0), time.Now(), false
	for _, p := range list {
		if p.Type != profile.TypeRemote || !p.AutoUpdate {
			continue
		}
		interval := max(intervalOrDefault(p.AutoUpdateInterval), minUpdateInterval)
		wait := p.LastUpdated.Add(interval).Sub(now)
		if wait <= 0 {
			opCtx, cancel := context.WithTimeout(ctx, 2*fetchTimeout)
			err := b.updateRemote(opCtx, p.ID)
			cancel()
			if ctx.Err() != nil {
				return 0
			}
			if err != nil {
				log.Printf("update profile %s failed: %v", p.Name, err)
			}
			updated = true // the attempt moves lastUpdated, or retries next round
			wait = interval
		}
		if next == 0 || wait < next {
			next = wait
		}
	}
	if updated && notify != nil {
		notify()
	}
	if next == 0 {
		return defaultUpdateInterval
	}
	return max(next, time.Minute)
}
