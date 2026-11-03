package networkcheck

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/rpc/gen/desktop"
)

func TestConfigIsAuthenticatedLoopbackOnly(t *testing.T) {
	config, err := BuildLoopbackConfig(41829, "temporary-user", "temporary-password")
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err = json.Unmarshal([]byte(config), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 4 || parsed["dns"] != nil || parsed["experimental"] != nil {
		t.Fatalf("unexpected config: %s", config)
	}
	inbounds := parsed["inbounds"].([]any)
	inbound := inbounds[0].(map[string]any)
	if len(inbounds) != 1 || inbound["type"] != "mixed" || inbound["listen"] != "127.0.0.1" || inbound["set_system_proxy"] != false || inbound["listen_port"] != float64(41829) {
		t.Fatalf("unsafe inbound: %v", inbound)
	}
	if !reflect.DeepEqual(inbound["users"], []any{map[string]any{"username": "temporary-user", "password": "temporary-password"}}) {
		t.Fatalf("missing auth: %v", inbound)
	}
	for _, port := range []int{-1, 0, 80, 1023, 65536} {
		if _, err := BuildLoopbackConfig(port, "u", "p"); err == nil {
			t.Fatalf("accepted port %d", port)
		}
	}
	if _, err := BuildLoopbackConfig(12345, "", ""); err == nil {
		t.Fatal("accepted empty credentials")
	}
}

func TestURLGuardRejectsExternalAndAmbiguousTargets(t *testing.T) {
	for _, raw := range []string{"https://127.0.0.1:12345/", "http://localhost:12345/", "http://example.com:12345/", "http://127.0.0.2:12345/", "http://127.0.0.1:80/", "http://u:p@127.0.0.1:12345/", "http://127.0.0.1:12345/?target=external", "http://[::1]:12345/"} {
		if localHTTPURL(raw) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if err := localHTTPURL("http://127.0.0.1:12345/random-token"); err != nil {
		t.Fatal(err)
	}
}

type ownedServer struct {
	desktop.UnimplementedDesktopServiceServer
	daemon.UnimplementedStartedServiceServer
	daemon.UnimplementedManagedServiceServer
	owner desktop.DaemonOwnership
	state daemon.ServiceStatus_Type
}

func (f *ownedServer) GetDaemonInfo(context.Context, *emptypb.Empty) (*desktop.DaemonInfo, error) {
	return &desktop.DaemonInfo{Ownership: f.owner}, nil
}
func (f *ownedServer) SubscribeServiceStatus(_ *emptypb.Empty, s grpc.ServerStreamingServer[daemon.ServiceStatus]) error {
	return s.Send(&daemon.ServiceStatus{Status: f.state})
}

func TestFakeGRPCRefusesOtherOwnerAndRunningCore(t *testing.T) {
	for _, f := range []*ownedServer{
		{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_OTHER},
		{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, state: daemon.ServiceStatus_STARTED},
	} {
		listener := bufconn.Listen(1024 * 1024)
		server := grpc.NewServer()
		desktop.RegisterDesktopServiceServer(server, f)
		daemon.RegisterStartedServiceServer(server, f)
		daemon.RegisterManagedServiceServer(server, f)
		go server.Serve(listener)
		conn, err := grpc.NewClient("passthrough:///networkcheck-fake", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		var report Report
		lifecycle(ctx, conn, &report, "must-not-start", func(context.Context) error { t.Error("existing core reached proxy request"); return nil })
		cancel()
		conn.Close()
		server.Stop()
		listener.Close()
		if report.Error == "" || report.Claimed || report.CoreStarted || report.CleanupAttempted || report.TrafficVerified {
			t.Fatalf("unsafe report: %+v", report)
		}
	}
}
