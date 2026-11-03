package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/ui/shell"
)

// watchConnections streams connection events at the Apple client's one
// second status interval. The shell keeps the connection table.
func (r runtimeReader) watchConnections(ctx context.Context, emit func(shell.ConnectionEvents)) error {
	a, err := r.probe(ctx)
	if err != nil {
		return err
	}
	if !a.Available {
		return fmt.Errorf("connections unavailable: %s", a.Reason)
	}
	stream, err := d.NewStartedServiceClient(r.conn).SubscribeConnections(ctx, &d.SubscribeConnectionsRequest{Interval: int64(time.Second)})
	if err != nil {
		return err
	}
	for {
		frame, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return errors.New("connections stream ended")
		}
		if err != nil {
			return err
		}
		out := shell.ConnectionEvents{Reset: frame.GetReset_(), Events: make([]shell.ConnectionEvent, 0, len(frame.GetEvents()))}
		for _, e := range frame.GetEvents() {
			if e == nil {
				continue
			}
			event := shell.ConnectionEvent{ID: e.GetId(), UplinkDelta: e.GetUplinkDelta(), DownlinkDelta: e.GetDownlinkDelta(), ClosedAt: e.GetClosedAt()}
			switch e.GetType() {
			case d.ConnectionEventType_CONNECTION_EVENT_NEW:
				event.Type = shell.ConnectionNew
			case d.ConnectionEventType_CONNECTION_EVENT_UPDATE:
				event.Type = shell.ConnectionUpdate
			case d.ConnectionEventType_CONNECTION_EVENT_CLOSED:
				event.Type = shell.ConnectionClosed
			default:
				continue
			}
			if c := e.GetConnection(); c != nil {
				event.Connection = &shell.Connection{
					ID: c.GetId(), Inbound: c.GetInbound(), InboundType: c.GetInboundType(), IPVersion: int(c.GetIpVersion()),
					Network: c.GetNetwork(), Source: c.GetSource(), Destination: c.GetDestination(), Domain: c.GetDomain(),
					Protocol: c.GetProtocol(), User: c.GetUser(), FromOutbound: c.GetFromOutbound(),
					CreatedAt: c.GetCreatedAt(), ClosedAt: c.GetClosedAt(),
					Uplink: c.GetUplink(), Downlink: c.GetDownlink(), UplinkTotal: c.GetUplinkTotal(), DownlinkTotal: c.GetDownlinkTotal(),
					Rule: c.GetRule(), Outbound: c.GetOutbound(), OutboundType: c.GetOutboundType(), Chain: c.GetChainList(),
				}
			}
			out.Events = append(out.Events, event)
		}
		emit(out)
	}
}

func (r runtimeReader) closeConnection(ctx context.Context, id string) error {
	if err := r.requireRunning(ctx); err != nil {
		return err
	}
	_, err := d.NewStartedServiceClient(r.conn).CloseConnection(ctx, &d.CloseConnectionRequest{Id: id})
	return err
}

func (r runtimeReader) closeAllConnections(ctx context.Context) error {
	if err := r.requireRunning(ctx); err != nil {
		return err
	}
	_, err := d.NewStartedServiceClient(r.conn).CloseAllConnections(ctx, &emptypb.Empty{})
	return err
}

func (r runtimeReader) requireRunning(ctx context.Context) error {
	a, err := r.probe(ctx)
	if err != nil {
		return err
	}
	if !a.Available {
		return errors.New("core is not owned and running")
	}
	return nil
}

func (b *Backend) watchConnections(ctx context.Context, emit func(shell.ConnectionEvents)) error {
	return b.withReader(ctx, func(r runtimeReader) error { return r.watchConnections(ctx, emit) })
}
func (b *Backend) closeConnection(ctx context.Context, id string) error {
	return b.withReader(ctx, func(r runtimeReader) error { return r.closeConnection(ctx, id) })
}
func (b *Backend) closeAllConnections(ctx context.Context) error {
	return b.withReader(ctx, func(r runtimeReader) error { return r.closeAllConnections(ctx) })
}
