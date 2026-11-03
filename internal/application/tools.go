package application

import (
	"context"
	"errors"
	"io"

	"google.golang.org/protobuf/types/known/emptypb"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/ui/shell"
)

// The Apple client's Network tools run in the started core: its outbound
// carries the test. Each call re-checks that this user owns a running core.
func (r runtimeReader) startedClient(ctx context.Context) (d.StartedServiceClient, error) {
	a, err := r.probe(ctx)
	if err != nil {
		return nil, err
	}
	if !a.Available {
		return nil, errors.New("core is not owned and running")
	}
	return d.NewStartedServiceClient(r.conn), nil
}

// outbounds reads the first frame of the outbound list, as the Apple
// client's outbound picker shows it.
func (b *Backend) outbounds(ctx context.Context) ([]string, error) {
	var out []string
	err := b.withReader(ctx, func(r runtimeReader) error {
		client, err := r.startedClient(ctx)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		stream, err := client.SubscribeOutbounds(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}
		frame, err := stream.Recv()
		if err != nil {
			return err
		}
		for _, o := range frame.GetOutbounds() {
			out = append(out, o.GetTag())
		}
		return nil
	})
	return out, err
}

func (b *Backend) networkQuality(ctx context.Context, req shell.NetworkQualityRequest, emit func(shell.NetworkQualityProgress)) error {
	return b.withReader(ctx, func(r runtimeReader) error {
		client, err := r.startedClient(ctx)
		if err != nil {
			return err
		}
		stream, err := client.StartNetworkQualityTest(ctx, &d.NetworkQualityTestRequest{
			ConfigURL: req.ConfigURL, OutboundTag: req.Outbound, Serial: req.Serial,
			MaxRuntimeSeconds: int32(req.MaxRuntimeSeconds), Http3: req.HTTP3,
		})
		if err != nil {
			return err
		}
		for {
			p, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			emit(shell.NetworkQualityProgress{
				Phase: int(p.GetPhase()), IdleLatencyMS: int(p.GetIdleLatencyMs()),
				Download: p.GetDownloadCapacity(), Upload: p.GetUploadCapacity(),
				DownloadRPM: int(p.GetDownloadRPM()), UploadRPM: int(p.GetUploadRPM()),
				DownloadAccuracy: int(p.GetDownloadCapacityAccuracy()), UploadAccuracy: int(p.GetUploadCapacityAccuracy()),
				DownloadRPMAccuracy: int(p.GetDownloadRPMAccuracy()), UploadRPMAccuracy: int(p.GetUploadRPMAccuracy()),
				Final: p.GetIsFinal(), Error: p.GetError(),
			})
			if p.GetIsFinal() {
				return nil
			}
		}
	})
}

func (b *Backend) stunTest(ctx context.Context, req shell.STUNRequest, emit func(shell.STUNProgress)) error {
	return b.withReader(ctx, func(r runtimeReader) error {
		client, err := r.startedClient(ctx)
		if err != nil {
			return err
		}
		stream, err := client.StartSTUNTest(ctx, &d.STUNTestRequest{Server: req.Server, OutboundTag: req.Outbound})
		if err != nil {
			return err
		}
		for {
			p, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return err
			}
			emit(shell.STUNProgress{
				Phase: int(p.GetPhase()), ExternalAddr: p.GetExternalAddr(), LatencyMS: int(p.GetLatencyMs()),
				NATMapping: int(p.GetNatMapping()), NATFiltering: int(p.GetNatFiltering()),
				NATTypeSupported: p.GetNatTypeSupported(), Final: p.GetIsFinal(), Error: p.GetError(),
			})
			if p.GetIsFinal() {
				return nil
			}
		}
	})
}
