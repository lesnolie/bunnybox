package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/rpc/gen/desktop"
)

const runtimeReadTimeout = 3 * time.Second
const runtimeRPCTimeout = 1500 * time.Millisecond

// Availability describes whether a real current snapshot was read. Available
// never means merely that the daemon can be claimed. Missing data must be shown
// as unavailable, not presented as zero traffic or an empty successful test.
type Availability struct {
	Available bool   `json:"available"`
	Ownership string `json:"ownership"`        // caller, available, other, or unknown
	Reason    string `json:"reason,omitempty"` // unclaimed, other_owner, core_idle, etc.
}
type MetricsSnapshot struct {
	Availability
	MemoryBytes      uint64 `json:"memoryBytes"`
	Goroutines       int32  `json:"goroutines"`
	ConnectionsIn    int32  `json:"connectionsIn"`
	ConnectionsOut   int32  `json:"connectionsOut"`
	TrafficAvailable bool   `json:"trafficAvailable"`
	// The upstream first status frame contains totals but no measured interval.
	// Uplink/Downlink preserve the wire values; RateAvailable is false for these
	// one-frame snapshots. A UI must not label them as a measured bytes/sec rate.
	RateAvailable bool  `json:"rateAvailable"`
	Uplink        int64 `json:"uplink"`
	Downlink      int64 `json:"downlink"`
	UplinkTotal   int64 `json:"uplinkTotal"`
	DownlinkTotal int64 `json:"downlinkTotal"`
}
type GroupItem struct {
	Tag         string `json:"tag"`
	Type        string `json:"type"`
	URLTestTime int64  `json:"urlTestTime"`
	DelayMS     int32  `json:"delayMS"` // existing upstream result; never runs a test
}
type Group struct {
	Tag        string      `json:"tag"`
	Type       string      `json:"type"`
	Selectable bool        `json:"selectable"`
	Selected   string      `json:"selected"`
	Expanded   bool        `json:"expanded"`
	Items      []GroupItem `json:"items"`
}
type GroupsSnapshot struct {
	Availability
	Groups []Group `json:"groups"`
}
type ModeSnapshot struct {
	Availability
	Current string   `json:"current"`
	Options []string `json:"options"`
}
type RuntimeSnapshot struct {
	Metrics MetricsSnapshot `json:"metrics"`
	Groups  GroupsSnapshot  `json:"groups"`
	Mode    ModeSnapshot    `json:"mode"`
}

// runtimeReader is transport-only and independently testable with fake gRPC.
// It has no ClaimService, StartService, URLTest or SelectOutbound call path.
type runtimeReader struct {
	conn    grpc.ClientConnInterface
	timeout time.Duration
}

func (b *Backend) runtimeReader(ctx context.Context) (runtimeReader, error) {
	if b.rpc == nil {
		return runtimeReader{}, errors.New("runtime RPC client is unavailable")
	}
	conn, e := b.rpc.Daemon(ctx)
	return runtimeReader{conn, runtimeRPCTimeout}, e
}
func runtimeFailure(a Availability, e error) Availability {
	a.Available = false
	switch status.Code(e) {
	case codes.DeadlineExceeded:
		a.Reason = "timeout"
	case codes.Canceled:
		a.Reason = "canceled"
	case codes.PermissionDenied:
		a.Reason = "permission_denied"
	default:
		if errors.Is(e, context.DeadlineExceeded) {
			a.Reason = "timeout"
		} else if errors.Is(e, context.Canceled) {
			a.Reason = "canceled"
		} else {
			a.Reason = "read_failed"
		}
	}
	return a
}
func (r runtimeReader) probe(ctx context.Context) (Availability, error) {
	a := Availability{Ownership: "unknown"}
	rpcCtx, cancel := context.WithTimeout(ctx, r.timeout)
	info, e := desktop.NewDesktopServiceClient(r.conn).GetDaemonInfo(rpcCtx, &emptypb.Empty{})
	cancel()
	if e != nil {
		return runtimeFailure(a, e), fmt.Errorf("read daemon ownership: %w", e)
	}
	switch info.GetOwnership() {
	case desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE:
		a.Ownership = "available"
		a.Reason = "unclaimed"
		return a, nil
	case desktop.DaemonOwnership_DAEMON_OWNERSHIP_OTHER:
		a.Ownership = "other"
		a.Reason = "other_owner"
		return a, nil
	case desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER:
		a.Ownership = "caller"
	default:
		a.Reason = "unknown_ownership"
		return a, errors.New("unknown daemon ownership")
	}
	return r.coreState(ctx, a)
}

// coreState completes a caller-owned Availability from the first service-status
// frame. Groups waits for a started core upstream, so an owned idle/stopping
// core must return here without waiting for startup.
func (r runtimeReader) coreState(ctx context.Context, a Availability) (Availability, error) {
	rpcCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	stream, e := d.NewStartedServiceClient(r.conn).SubscribeServiceStatus(rpcCtx, &emptypb.Empty{})
	if e != nil {
		return runtimeFailure(a, e), fmt.Errorf("read core status: %w", e)
	}
	state, e := stream.Recv()
	if e != nil {
		return runtimeFailure(a, e), fmt.Errorf("read core status: %w", e)
	}
	if state.GetStatus() != d.ServiceStatus_STARTED {
		switch state.GetStatus() {
		case d.ServiceStatus_IDLE:
			a.Reason = "core_idle"
		case d.ServiceStatus_STARTING:
			a.Reason = "core_starting"
		case d.ServiceStatus_STOPPING:
			a.Reason = "core_stopping"
		case d.ServiceStatus_FATAL:
			a.Reason = "core_failed"
		default:
			a.Reason = "unknown_core_state"
		}
		return a, nil
	}
	a.Available = true
	return a, nil
}
func (r runtimeReader) metrics(ctx context.Context) (MetricsSnapshot, error) {
	a, e := r.probe(ctx)
	if e != nil || !a.Available {
		return MetricsSnapshot{Availability: a}, e
	}
	return r.readMetrics(ctx, a)
}

// readMetrics reads one status frame for an already probed, started core.
func (r runtimeReader) readMetrics(ctx context.Context, a Availability) (MetricsSnapshot, error) {
	out := MetricsSnapshot{Availability: a}
	rpcCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	stream, e := d.NewStartedServiceClient(r.conn).SubscribeStatus(rpcCtx, &d.SubscribeStatusRequest{Interval: int64(time.Second)})
	if e != nil {
		out.Availability = runtimeFailure(a, e)
		return out, fmt.Errorf("read metrics: %w", e)
	}
	frame, e := stream.Recv()
	if e != nil {
		out.Availability = runtimeFailure(a, e)
		return out, fmt.Errorf("read metrics: %w", e)
	}
	out.MemoryBytes = frame.GetMemory()
	out.Goroutines = frame.GetGoroutines()
	out.ConnectionsIn = frame.GetConnectionsIn()
	out.ConnectionsOut = frame.GetConnectionsOut()
	out.TrafficAvailable = frame.GetTrafficAvailable()
	out.Uplink = frame.GetUplink()
	out.Downlink = frame.GetDownlink()
	out.UplinkTotal = frame.GetUplinkTotal()
	out.DownlinkTotal = frame.GetDownlinkTotal()
	return out, nil
}
func (r runtimeReader) groups(ctx context.Context) (GroupsSnapshot, error) {
	a, e := r.probe(ctx)
	out := GroupsSnapshot{Availability: a}
	if e != nil || !a.Available {
		return out, e
	}
	rpcCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	stream, e := d.NewStartedServiceClient(r.conn).SubscribeGroups(rpcCtx, &emptypb.Empty{})
	if e != nil {
		out.Availability = runtimeFailure(a, e)
		return out, fmt.Errorf("read groups: %w", e)
	}
	frame, e := stream.Recv()
	if e != nil {
		out.Availability = runtimeFailure(a, e)
		return out, fmt.Errorf("read groups: %w", e)
	}
	out.Groups = groupsFromFrame(frame)
	return out, nil
}
func groupsFromFrame(frame *d.Groups) []Group {
	groups := make([]Group, 0, len(frame.GetGroup()))
	for _, g := range frame.GetGroup() {
		if g == nil {
			continue
		}
		items := make([]GroupItem, 0, len(g.GetItems()))
		for _, item := range g.GetItems() {
			if item == nil {
				continue
			}
			items = append(items, GroupItem{item.GetTag(), item.GetType(), item.GetUrlTestTime(), item.GetUrlTestDelay()})
		}
		groups = append(groups, Group{g.GetTag(), g.GetType(), g.GetSelectable(), g.GetSelected(), g.GetIsExpand(), items})
	}
	return groups
}
func (r runtimeReader) mode(ctx context.Context) (ModeSnapshot, error) {
	a, e := r.probe(ctx)
	if e != nil || !a.Available {
		return ModeSnapshot{Availability: a}, e
	}
	return r.readMode(ctx, a)
}

// readMode reads the Clash mode for an already probed, started core.
func (r runtimeReader) readMode(ctx context.Context, a Availability) (ModeSnapshot, error) {
	out := ModeSnapshot{Availability: a}
	rpcCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	frame, e := d.NewStartedServiceClient(r.conn).GetClashModeStatus(rpcCtx, &emptypb.Empty{})
	if status.Code(e) == codes.NotFound {
		out.Available = false
		out.Reason = "mode_unavailable"
		return out, nil
	}
	if e != nil {
		out.Availability = runtimeFailure(a, e)
		return out, fmt.Errorf("read clash mode: %w", e)
	}
	out.Current = frame.GetCurrentMode()
	out.Options = append([]string{}, frame.GetModeList()...)
	return out, nil
}

// Metrics, Groups and Mode are independent bounded reads, so callers need not
// serialize an unrelated node grid refresh behind a metrics request.
func (b *Backend) Metrics(ctx context.Context) (MetricsSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, runtimeReadTimeout)
	defer cancel()
	r, e := b.runtimeReader(ctx)
	if e != nil {
		return MetricsSnapshot{Availability: runtimeFailure(Availability{Ownership: "unknown"}, e)}, e
	}
	return r.metrics(ctx)
}
func (b *Backend) Groups(ctx context.Context) (GroupsSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, runtimeReadTimeout)
	defer cancel()
	r, e := b.runtimeReader(ctx)
	if e != nil {
		return GroupsSnapshot{Availability: runtimeFailure(Availability{Ownership: "unknown"}, e)}, e
	}
	return r.groups(ctx)
}
func (b *Backend) Mode(ctx context.Context) (ModeSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, runtimeReadTimeout)
	defer cancel()
	r, e := b.runtimeReader(ctx)
	if e != nil {
		return ModeSnapshot{Availability: runtimeFailure(Availability{Ownership: "unknown"}, e)}, e
	}
	return r.mode(ctx)
}

// Runtime preserves successful sections if another read fails. It performs only
// read RPCs; no ownership claim, service start, probing or network mutation.
func (b *Backend) Runtime(ctx context.Context) (RuntimeSnapshot, error) {
	var out RuntimeSnapshot
	var wg sync.WaitGroup
	var metricsErr, groupsErr, modeErr error
	wg.Go(func() { out.Metrics, metricsErr = b.Metrics(ctx) })
	wg.Go(func() { out.Groups, groupsErr = b.Groups(ctx) })
	wg.Go(func() { out.Mode, modeErr = b.Mode(ctx) })
	wg.Wait()
	return out, errors.Join(metricsErr, groupsErr, modeErr)
}
