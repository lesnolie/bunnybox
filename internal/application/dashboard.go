package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/ui/shell"
)

func (r runtimeReader) watchMetrics(ctx context.Context, emit func(MetricsSnapshot)) error {
	a, err := r.probe(ctx)
	if err != nil {
		return err
	}
	if !a.Available {
		return fmt.Errorf("metrics unavailable: %s", a.Reason)
	}
	stream, err := d.NewStartedServiceClient(r.conn).SubscribeStatus(ctx, &d.SubscribeStatusRequest{Interval: int64(time.Second)})
	if err != nil {
		return err
	}
	first := true
	for {
		frame, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("metrics stream ended")
			}
			return err
		}
		emit(MetricsSnapshot{Availability: a, MemoryBytes: frame.GetMemory(), Goroutines: frame.GetGoroutines(), ConnectionsIn: frame.GetConnectionsIn(), ConnectionsOut: frame.GetConnectionsOut(), TrafficAvailable: frame.GetTrafficAvailable(), RateAvailable: !first && frame.GetTrafficAvailable(), Uplink: frame.GetUplink(), Downlink: frame.GetDownlink(), UplinkTotal: frame.GetUplinkTotal(), DownlinkTotal: frame.GetDownlinkTotal()})
		first = false
	}
}
func (b *Backend) watchMetrics(ctx context.Context, emit func(shell.Metrics)) error {
	r, err := b.runtimeReader(ctx)
	if err != nil {
		return err
	}
	err = r.watchMetrics(ctx, func(m MetricsSnapshot) { emit(*uiMetrics(m)) })
	if status.Code(err) == codes.Unavailable {
		// The worker relay dropped (e.g. it exited); the core may still run.
		return fmt.Errorf("与后台服务的连接已中断，请点击“检测服务”重新连接（%w）", err)
	}
	return err
}

// These mutations are wired only to an explicit UI click. Ownership and
// capability are checked again at the RPC boundary; never claim/start here.
func (r runtimeReader) setHTTPProxy(ctx context.Context, enabled bool) error {
	a, err := r.probe(ctx)
	if err != nil {
		return err
	}
	if !a.Available {
		return errors.New("core is not owned and running")
	}
	client := d.NewManagedServiceClient(r.conn)
	proxy, err := client.GetSystemProxyStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	if !proxy.GetAvailable() {
		return errors.New("system HTTP proxy is unavailable")
	}
	_, err = client.SetSystemProxyEnabled(ctx, &d.SetSystemProxyEnabledRequest{Enabled: enabled})
	return err
}
func (b *Backend) setHTTPProxy(ctx context.Context, enabled bool) error {
	r, err := b.runtimeReader(ctx)
	if err != nil {
		return err
	}
	return r.setHTTPProxy(ctx, enabled)
}
func (r runtimeReader) setMode(ctx context.Context, mode string) error {
	snapshot, err := r.mode(ctx)
	if err != nil {
		return err
	}
	if !snapshot.Available || !slices.Contains(snapshot.Options, mode) {
		return errors.New("clash mode is unavailable")
	}
	_, err = d.NewStartedServiceClient(r.conn).SetClashMode(ctx, &d.ClashMode{Mode: mode})
	return err
}
func (b *Backend) setMode(ctx context.Context, mode string) error {
	r, err := b.runtimeReader(ctx)
	if err != nil {
		return err
	}
	return r.setMode(ctx, mode)
}
