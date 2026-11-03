package application

import (
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/types/known/emptypb"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/ui/shell"
)

// watchLogs streams the core log like the Apple client's CommandClient: the
// first frame replays the service's saved lines with Reset set, later frames
// append, and a frame with Reset and no lines means the log was cleared.
func (r runtimeReader) watchLogs(ctx context.Context, emit func(shell.LogFrame)) error {
	a, err := r.probe(ctx)
	if err != nil {
		return err
	}
	if !a.Available {
		return fmt.Errorf("logs unavailable: %s", a.Reason)
	}
	client := d.NewStartedServiceClient(r.conn)
	// The default level filters the view until the user picks one. The
	// upstream call fails without a running instance; Info is its default.
	level := int(d.LogLevel_INFO)
	if l, err := client.GetDefaultLogLevel(ctx, &emptypb.Empty{}); err == nil {
		level = int(l.GetLevel())
	}
	stream, err := client.SubscribeLog(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	for {
		frame, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return errors.New("log stream ended")
		}
		if err != nil {
			return err
		}
		out := shell.LogFrame{Reset: frame.GetReset_(), DefaultLevel: level, Lines: make([]shell.LogLine, 0, len(frame.GetMessages()))}
		for _, m := range frame.GetMessages() {
			if m != nil {
				out.Lines = append(out.Lines, shell.LogLine{Level: int(m.GetLevel()), Message: m.GetMessage()})
			}
		}
		emit(out)
	}
}

// clearLogs empties the service's saved lines; subscribers receive a reset.
func (r runtimeReader) clearLogs(ctx context.Context) error {
	a, err := r.probe(ctx)
	if err != nil {
		return err
	}
	if !a.Available {
		return errors.New("core is not owned and running")
	}
	_, err = d.NewStartedServiceClient(r.conn).ClearLogs(ctx, &emptypb.Empty{})
	return err
}

func (b *Backend) watchLogs(ctx context.Context, emit func(shell.LogFrame)) error {
	return b.withReader(ctx, func(r runtimeReader) error { return r.watchLogs(ctx, emit) })
}
func (b *Backend) clearLogs(ctx context.Context) error {
	return b.withReader(ctx, func(r runtimeReader) error { return r.clearLogs(ctx) })
}
