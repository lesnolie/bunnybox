package client

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"singboxwin.local/client/internal/rpc/worker"
)

func TestTransportAuthorityAndLocale(t *testing.T) {
	for _, authority := range []string{"sing-box-worker", "sing-box"} {
		t.Run(authority, func(t *testing.T) {
			listener := bufconn.Listen(64 * 1024)
			defer listener.Close()
			seen := make(chan metadata.MD, 2)
			server := grpc.NewServer(
				grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
					md, _ := metadata.FromIncomingContext(ctx)
					seen <- md
					return handler(ctx, req)
				}),
				grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
					md, _ := metadata.FromIncomingContext(ss.Context())
					seen <- md
					return handler(srv, ss)
				}),
			)
			grpc_health_v1.RegisterHealthServer(server, health.NewServer())
			go server.Serve(listener)
			defer server.Stop()
			lang := "zh-Hans"
			c := New(nil, Config{Language: func() string { return lang }})
			c.dial = func(ctx context.Context, pipe string) (net.Conn, error) {
				if pipe != "expected-pipe" {
					t.Error(pipe)
				}
				return listener.DialContext(ctx)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			conn, err := c.dialTransport(ctx, new(worker.Process), "expected-pipe", authority)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			h := grpc_health_v1.NewHealthClient(conn)
			ctx = metadata.AppendToOutgoingContext(ctx, "accept-language", "stale", "custom", "kept")
			if _, err := h.Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err != nil {
				t.Fatal(err)
			}
			assertMetadata(t, <-seen, authority, "zh-Hans")
			lang = "en-US"
			stream, err := h.Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stream.Recv(); err != nil {
				t.Fatal(err)
			}
			assertMetadata(t, <-seen, authority, "en-US")
		})
	}
}

func assertMetadata(t *testing.T, md metadata.MD, authority, lang string) {
	t.Helper()
	for key, want := range map[string]string{":authority": authority, "accept-language": lang, "custom": "kept"} {
		got := md.Get(key)
		if len(got) != 1 || got[0] != want {
			t.Errorf("metadata %s = %v, want %s", key, got, want)
		}
	}
}

func TestTransportDoesNotClaimConnectionWithoutServer(t *testing.T) {
	c := New(nil, Config{DialTimeout: 30 * time.Millisecond})
	c.dial = func(ctx context.Context, _ string) (net.Conn, error) { <-ctx.Done(); return nil, ctx.Err() }
	conn, err := c.dialTransport(context.Background(), new(worker.Process), "missing", "sing-box")
	if err == nil || conn != nil {
		t.Fatalf("connection=%v error=%v", conn, err)
	}
}
