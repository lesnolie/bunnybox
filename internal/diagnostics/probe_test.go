package diagnostics

import (
	"context"
	"errors"
	"singboxwin.local/client/internal/daemon"
	"testing"
)

func TestFailedServiceRetainsReadOnlyEvidence(t *testing.T) {
	report := Report{Executable: "installed-daemon", WorkerReady: true, ApplicationRPC: true}
	want := &daemon.CheckError{Stage: "service_not_installed", Cause: errors.New("not installed")}
	err := inspectService(context.Background(), &report, func(_ context.Context, path string) (daemon.Info, error) {
		if path != report.Executable {
			t.Fatal(path)
		}
		return daemon.Info{Name: daemon.ServiceName, State: daemon.NotInstalled, CLIExitCode: 3}, want
	})
	if !errors.Is(err, want) || daemon.FailedStage(err) != "service_not_installed" {
		t.Fatal(err)
	}
	if report.Service == nil || report.Service.State != daemon.NotInstalled || !report.ApplicationRPC || report.DaemonRPC {
		t.Fatalf("%+v", report)
	}
}

func TestVersionMismatchIsNotReportedAsConnectedCore(t *testing.T) {
	report := Report{Version: "1.14.4"}
	err := recordDaemonInfo(&report, "1.14.3", "AVAILABLE")
	if err == nil || daemon.FailedStage(err) != "daemon_version" || report.VersionMatch == nil || *report.VersionMatch {
		t.Fatalf("%+v %v", report, err)
	}
	if !report.DaemonRPC || report.DaemonVersion != "1.14.3" || report.Ownership != "AVAILABLE" {
		t.Fatal(report)
	}
	if err := recordDaemonInfo(&report, "1.14.4", "CALLER"); err != nil || !*report.VersionMatch {
		t.Fatalf("%+v %v", report, err)
	}
}
