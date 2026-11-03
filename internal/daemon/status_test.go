package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
)

// Only fake subprocess stdout/exit status is used. These tests never invoke
// service installation, SCM mutations, registry writes, or an official worker.
func TestMain(m *testing.M) {
	if mode := os.Getenv("SINGBOX_DAEMON_TEST_MODE"); mode != "" {
		if mode == "version" {
			fmt.Print("sing-box-daemon version 1.14.4\ncore api version 1\n")
			os.Exit(0)
		}
		if len(os.Args) != 3 || os.Args[1] != "service" || os.Args[2] != "status" {
			os.Exit(99)
		}
		code, _ := strconv.Atoi(mode)
		fmt.Println("fixture service status")
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func TestOfficialCLIExitCodes(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		code  int
		state State
	}{{0, Running}, {2, Stopped}, {3, NotInstalled}, {1, Unknown}, {7, Unknown}} {
		t.Run(strconv.Itoa(tc.code), func(t *testing.T) {
			t.Setenv("SINGBOX_DAEMON_TEST_MODE", strconv.Itoa(tc.code))
			state, code, output, err := cliStatus(context.Background(), exe)
			if state != tc.state || code != tc.code || output != "fixture service status" {
				t.Fatalf("%s %d %q", state, code, output)
			}
			if (err != nil) != (tc.state == Unknown) {
				t.Fatalf("unexpected error %v", err)
			}
		})
	}
}

func TestVersionParsingAndComparison(t *testing.T) {
	t.Setenv("SINGBOX_DAEMON_TEST_MODE", "version")
	exe, _ := os.Executable()
	version, err := BinaryVersion(context.Background(), exe)
	if err != nil || version != "1.14.4" {
		t.Fatalf("%q %v", version, err)
	}
	if err := CheckVersion(version, "1.14.4"); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"1.14.3", "", "1.14.4-dev"} {
		if stage := FailedStage(CheckVersion(version, v)); stage != "daemon_version" {
			t.Fatal(stage)
		}
	}
	for _, output := range []string{"core api version 1", "sing-box-daemon version", "junk"} {
		if _, err := ParseVersion(output); err == nil {
			t.Fatal(output)
		}
	}
}

func validInfo() Info {
	return Info{State: Running, NativeState: "running", PathMatches: true, Account: "LocalSystem", WorkingDirectory: `C:\ProgramData\sing-box-daemon`, Layout: Layout{RegistryPresent: true, VersionPresent: true, Version: 2, DaemonDataDirectory: `C:\ProgramData\sing-box-daemon`}}
}

func TestValidationFailures(t *testing.T) {
	for _, tc := range []struct {
		name, stage string
		alter       func(*Info)
	}{
		{"no_registry", "installation_layout", func(i *Info) { i.Layout.RegistryPresent = false }},
		{"no_layout_version", "installation_layout", func(i *Info) { i.Layout.VersionPresent = false }},
		{"wrong_layout_version", "installation_layout", func(i *Info) { i.Layout.Version = 1 }},
		{"stale_path", "service_path", func(i *Info) { i.PathMatches = false; i.Executable = `C:\old\sing-box-daemon.exe` }},
		{"wrong_account", "service_identity", func(i *Info) { i.Account = `NT AUTHORITY\LocalService` }},
		{"wrong_working_directory", "service_configuration", func(i *Info) { i.WorkingDirectory = `C:\other` }},
		{"stopped", "service_not_running", func(i *Info) { i.State = Stopped; i.NativeState = "stopped" }},
		{"stopping_between_queries", "service_not_running", func(i *Info) { i.NativeState = "stop_pending" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := validInfo()
			tc.alter(&info)
			err := validate(info)
			if err == nil || FailedStage(err) != tc.stage {
				t.Fatalf("%v", err)
			}
		})
	}
	if err := validate(validInfo()); err != nil {
		t.Fatal(err)
	}
}

func TestServiceArguments(t *testing.T) {
	valid := []string{`C:\Program Files\sing-box\resources\daemon\sing-box-daemon.exe`, "run", "--working-directory", `C:\ProgramData\sing-box-daemon`}
	var info Info
	if err := serviceArguments(&info, valid); err != nil {
		t.Fatal(err)
	}
	if info.Executable != valid[0] || info.WorkingDirectory != valid[3] {
		t.Fatal(info)
	}
	if err := serviceArguments(&info, append(valid, "--allow-unsafe-installation-directory-permissions")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, valid[:1], valid[:3], {"daemon.exe", "worker", "--working-directory", "dir"}, append(valid, "--unexpected")} {
		if err := serviceArguments(&info, args); err == nil {
			t.Fatal(args)
		}
	}
}

func TestInspectNotInstalledPrecedesMissingRegistry(t *testing.T) {
	info, err := inspect(context.Background(), "daemon.exe", checks{
		status: func(context.Context, string) (State, int, string, error) {
			return NotInstalled, 3, "not installed", nil
		},
		layout: func() (Layout, error) {
			t.Fatal("not installed must not require installation registry")
			return Layout{}, nil
		},
		service: func(*Info, string) error { t.Fatal("not installed must not query service config"); return nil },
	})
	if info.State != NotInstalled || FailedStage(err) != "service_not_installed" {
		t.Fatalf("%+v %v", info, err)
	}
}

func TestInspectStageFailures(t *testing.T) {
	for _, stage := range []string{"service_status", "installation_layout", "service_configuration", "service_not_running"} {
		t.Run(stage, func(t *testing.T) {
			check := checks{
				status: func(context.Context, string) (State, int, string, error) {
					if stage == "service_status" {
						return Unknown, 1, "failed", errors.New("denied")
					}
					return Stopped, 2, "stopped", nil
				},
				layout: func() (Layout, error) {
					if stage == "installation_layout" {
						return Layout{}, nil
					}
					return validInfo().Layout, nil
				},
				service: func(i *Info, _ string) error {
					if stage == "service_configuration" {
						return errors.New("SCM query denied")
					}
					layout := i.Layout
					*i = validInfo()
					i.Layout = layout
					i.State = Stopped
					i.NativeState = "stopped"
					return nil
				},
			}
			_, err := inspect(context.Background(), "daemon.exe", check)
			if err == nil || FailedStage(err) != stage {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestNativeStates(t *testing.T) {
	for code, want := range map[uint32]string{0: "unknown", 1: "stopped", 2: "start_pending", 3: "stop_pending", 4: "running", 5: "continue_pending", 6: "pause_pending", 7: "paused"} {
		if got := nativeState(code); got != want {
			t.Fatalf("%d: %s", code, got)
		}
	}
}
