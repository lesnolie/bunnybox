// Package application binds the native UI to local profiles and official RPCs.
package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"google.golang.org/protobuf/types/known/emptypb"
	"singboxwin.local/client/internal/daemon"
	"singboxwin.local/client/internal/profile"
	"singboxwin.local/client/internal/rpc/client"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/rpc/gen/desktop"
	"singboxwin.local/client/internal/rpc/worker"
	"singboxwin.local/client/internal/ui/shell"
)

type Backend struct {
	mu          sync.Mutex
	profiles    *profile.Service
	rpc         *client.Client
	executable  string
	wake        chan struct{}
	stopUpdater context.CancelFunc
}

func Open(ctx context.Context, directory string) (*Backend, error) {
	path, err := worker.InstalledExecutable()
	if err != nil {
		return nil, err
	}
	m, err := worker.New(worker.Config{Executable: path})
	if err != nil {
		return nil, err
	}
	b := &Backend{executable: path, wake: make(chan struct{}, 1)}
	b.rpc = client.New(m, client.Config{Language: func() string { return "zh-Hans" }})
	b.profiles, err = profile.Open(ctx, directory, profile.CheckFunc(b.check))
	if err != nil {
		b.rpc.Close() // also closes the worker manager
		return nil, err
	}
	return b, nil
}

// Close stops the worker transports (and the worker) before the profile store.
func (b *Backend) Close() {
	if b.stopUpdater != nil {
		b.stopUpdater()
	}
	b.rpc.Close()
	b.profiles.Close()
}
func (b *Backend) check(ctx context.Context, content string) error {
	conn, err := b.rpc.Application(ctx)
	if err != nil {
		return err
	}
	_, err = desktop.NewApplicationServiceClient(conn).CheckConfig(ctx, &desktop.ConfigContent{Content: content})
	return err
}
func id(value string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("invalid profile ID")
	}
	return n, nil
}
func (b *Backend) Callbacks() shell.Callbacks {
	return shell.Callbacks{
		List: func(ctx context.Context) ([]shell.Profile, error) {
			list, err := b.profiles.List(ctx)
			if err != nil {
				return nil, err
			}
			selected, err := b.profiles.Selected(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]shell.Profile, 0, len(list))
			for _, p := range list {
				out = append(out, uiProfile(p, selected != nil && selected.ID == p.ID))
			}
			return out, nil
		},
		Create: func(ctx context.Context, name, content string) error {
			if _, err := b.profiles.ImportJSON(ctx, name, content); err != nil {
				return err
			}
			return b.reload(ctx) // a new profile is selected, as in Apple's import
		},
		CreateRemote: func(ctx context.Context, name, address string, autoUpdate bool, interval int) error {
			if err := b.createRemote(ctx, name, address, autoUpdate, interval); err != nil {
				return err
			}
			return b.reload(ctx)
		},
		UpdateRemote: func(ctx context.Context, value string) error {
			n, err := id(value)
			if err != nil {
				return err
			}
			return b.updateRemote(ctx, n)
		},
		SaveProfile: func(ctx context.Context, value string, s shell.ProfileSettings) error {
			n, err := id(value)
			if err != nil {
				return err
			}
			return b.saveProfile(ctx, n, s)
		},
		EncodeProfile: func(ctx context.Context, value string) ([]byte, error) {
			n, err := id(value)
			if err != nil {
				return nil, err
			}
			return b.encodeProfile(ctx, n)
		},
		ProfileName:   b.profileName,
		ImportProfile: b.importProfile,
		SaveSettings:  b.saveSettings,
		StartAtLogin:  func(context.Context) (bool, error) { return startAtLogin() },
		SetStartAtLogin: func(_ context.Context, on bool) error {
			return setStartAtLogin(on)
		},
		CoreInfo: b.coreInfo,
		ReadContent: func(ctx context.Context, value string) (string, error) {
			n, err := id(value)
			if err != nil {
				return "", err
			}
			return b.profiles.ReadContent(ctx, n)
		},
		UpdateContent: func(ctx context.Context, value, content string) error {
			n, err := id(value)
			if err != nil {
				return err
			}
			if err = b.profiles.UpdateContent(ctx, n, content); err != nil {
				return err
			}
			return b.reloadIfSelected(ctx, n)
		},
		Select: func(ctx context.Context, value string) error {
			n, err := id(value)
			if err != nil {
				return err
			}
			if err = b.profiles.Select(ctx, n); err != nil {
				return err
			}
			return b.reload(ctx) // Apple's switchProfile
		},
		Delete: func(ctx context.Context, value string) error {
			n, err := id(value)
			if err != nil {
				return err
			}
			return b.profiles.Delete(ctx, n)
		},
		Status: b.status, Start: b.start, Stop: b.stop,
		WatchMetrics: b.watchMetrics, SetHTTPProxy: b.setHTTPProxy, SetMode: b.setMode,
		WatchGroups: b.watchGroups, SelectOutbound: b.selectOutbound, URLTest: b.urlTest, SetGroupExpand: b.setGroupExpand,
		WatchLogs: b.watchLogs, ClearLogs: b.clearLogs,
		WatchConnections: b.watchConnections, CloseConnection: b.closeConnection, CloseAllConnections: b.closeAllConnections,
		Outbounds: b.outbounds, NetworkQuality: b.networkQuality, STUNTest: b.stunTest,
		CheckUpdate: b.checkUpdate, InstallUpdate: b.installUpdate,
	}
}

// Reading status does not claim ownership, start a core or alter network settings.
func (b *Backend) status(ctx context.Context) (shell.ServiceState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var result shell.ServiceState
	if _, err := daemon.Inspect(ctx, b.executable); err != nil {
		return result, err
	}
	r, err := b.runtimeReader(ctx)
	if err != nil {
		return result, err
	}
	remote, err := desktop.NewDesktopServiceClient(r.conn).GetDaemonInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return result, err
	}
	version, err := daemon.BinaryVersion(ctx, b.executable)
	if err != nil {
		return result, err
	}
	if err = daemon.CheckVersion(version, remote.GetVersion()); err != nil {
		return result, err
	}
	result.Version = remote.GetVersion()
	switch remote.GetOwnership() {
	case desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE:
		result.Available = true
		result.Message = "available"
		return result, nil
	case desktop.DaemonOwnership_DAEMON_OWNERSHIP_OTHER:
		result.Message = "other_user"
		return result, nil
	case desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER:
	default:
		return result, errors.New("unknown daemon ownership")
	}
	result.Owned = true
	a, err := r.coreState(ctx, Availability{Ownership: "caller"})
	if err != nil {
		return result, err
	}
	result.Running = a.Available
	result.Available = a.Available || a.Reason == "core_idle"
	result.Message = a.Reason
	if result.Message == "" {
		result.Message = "core_running"
	}
	if !result.Running {
		return result, nil
	}
	// One probe serves every runtime read below; each read is still bounded.
	metrics, err := r.readMetrics(ctx, a)
	if err != nil {
		return result, err
	}
	if metrics.Available {
		result.Metrics = uiMetrics(metrics)
	}
	mode, err := r.readMode(ctx, a)
	if err != nil {
		return result, err
	}
	if mode.Available {
		result.ClashMode = mode.Current
		result.ModeOptions = mode.Options
	}
	proxy, err := d.NewManagedServiceClient(r.conn).GetSystemProxyStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return result, err
	}
	if proxy.GetAvailable() {
		enabled := proxy.GetEnabled()
		result.HTTPProxy = &enabled
	}
	return result, nil
}

func readableBytes(value uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	n := float64(value)
	unit := 0
	for n >= 1024 && unit < len(units)-1 {
		n /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", value, units[unit])
	}
	return fmt.Sprintf("%.1f %s", n, units[unit])
}
func uiMetrics(m MetricsSnapshot) *shell.Metrics {
	out := &shell.Metrics{Memory: readableBytes(m.MemoryBytes), Goroutines: strconv.FormatInt(int64(m.Goroutines), 10), Inbound: strconv.FormatInt(int64(m.ConnectionsIn), 10), Outbound: strconv.FormatInt(int64(m.ConnectionsOut), 10)}
	if m.TrafficAvailable {
		if m.UplinkTotal >= 0 {
			out.UploadTotal = readableBytes(uint64(m.UplinkTotal))
		}
		if m.DownlinkTotal >= 0 {
			out.DownloadTotal = readableBytes(uint64(m.DownlinkTotal))
		}
		if m.RateAvailable {
			out.RateAvailable = m.Uplink >= 0 && m.Downlink >= 0
			out.UploadBytes, out.DownloadBytes = float64(m.Uplink), float64(m.Downlink)
			if m.Uplink >= 0 {
				out.Upload = readableBytes(uint64(m.Uplink)) + "/s"
			}
			if m.Downlink >= 0 {
				out.Download = readableBytes(uint64(m.Downlink)) + "/s"
			}
		}
	}
	return out
}
func (b *Backend) start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	p, err := b.profiles.Selected(ctx)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.New("请先选择配置")
	}
	content, err := b.profiles.ReadContent(ctx, p.ID)
	if err != nil {
		return err
	}
	if err = b.check(ctx, content); err != nil {
		return err
	}
	if _, err = daemon.Inspect(ctx, b.executable); err != nil {
		return err
	}
	conn, err := b.rpc.Daemon(ctx)
	if err != nil {
		return err
	}
	dc := desktop.NewDesktopServiceClient(conn)
	info, err := dc.GetDaemonInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	if info.GetOwnership() != desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE && info.GetOwnership() != desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER {
		return errors.New("拒绝接管其他用户的服务")
	}
	if _, err = dc.ClaimService(ctx, &emptypb.Empty{}); err != nil {
		return err
	}
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := d.NewStartedServiceClient(conn).SubscribeServiceStatus(streamCtx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	state, err := stream.Recv()
	if err != nil {
		return err
	}
	if state.GetStatus() != d.ServiceStatus_IDLE {
		return errors.New("服务不处于空闲状态，不替换已运行配置")
	}
	_, err = dc.StartService(ctx, &desktop.StartServiceRequest{ConfigContent: content, Options: &desktop.StartOptions{OomKillerDisabled: true}})
	return err
}
func (b *Backend) stop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	conn, err := b.rpc.Daemon(ctx)
	if err != nil {
		return err
	}
	info, err := desktop.NewDesktopServiceClient(conn).GetDaemonInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	if info.GetOwnership() != desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER {
		return errors.New("只能停止本用户拥有的核心")
	}
	_, err = d.NewManagedServiceClient(conn).StopService(ctx, &emptypb.Empty{})
	return err
}
