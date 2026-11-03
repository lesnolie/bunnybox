// Package daemon provides read-only Windows service and installation diagnostics.
// It never creates, starts, stops, claims, or reconfigures a service.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const ServiceName = "sing-box-daemon"

type State string

const (
	Running      State = "running"
	Stopped      State = "stopped"
	NotInstalled State = "not_installed"
	Unknown      State = "unknown"
)

type Layout struct {
	RegistryPresent          bool   `json:"registry_present"`
	VersionPresent           bool   `json:"version_present"`
	Version                  uint64 `json:"version,omitempty"`
	ApplicationDataDirectory string `json:"application_data_directory,omitempty"`
	DaemonDataDirectory      string `json:"daemon_data_directory,omitempty"`
}

type Info struct {
	Name             string `json:"name"`
	State            State  `json:"state"`
	CLIExitCode      int    `json:"cli_exit_code"`
	CLIOutput        string `json:"cli_output,omitempty"`
	NativeState      string `json:"native_state,omitempty"`
	ProcessID        uint32 `json:"process_id,omitempty"`
	CommandLine      string `json:"command_line,omitempty"`
	Executable       string `json:"executable,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	Account          string `json:"account,omitempty"`
	PathMatches      bool   `json:"path_matches_current_installation"`
	Layout           Layout `json:"layout"`
}

type CheckError struct {
	Stage string
	Cause error
}

func (e *CheckError) Error() string { return e.Cause.Error() }
func (e *CheckError) Unwrap() error { return e.Cause }
func fail(stage, format string, args ...any) error {
	return &CheckError{Stage: stage, Cause: fmt.Errorf(format, args...)}
}

func FailedStage(err error) string {
	var check *CheckError
	if errors.As(err, &check) {
		return check.Stage
	}
	return "service_status"
}

// StateFromExitCode follows the official boxdd service status contract: 0 means
// running, 2 means every installed non-running state, and 3 means not installed.
func StateFromExitCode(code int) State {
	switch code {
	case 0:
		return Running
	case 2:
		return Stopped
	case 3:
		return NotInstalled
	default:
		return Unknown
	}
}

// ParseVersion extracts only the daemon version; the command also prints a
// distinct core API version line, which must not participate in comparison.
func ParseVersion(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "sing-box-daemon" && fields[1] == "version" {
			return fields[2], nil
		}
	}
	return "", errors.New("daemon version output has no sing-box-daemon version line")
}

func BinaryVersion(ctx context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "version")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("daemon version: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return ParseVersion(string(out))
}

func cliStatus(ctx context.Context, path string) (State, int, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "service", "status")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if ctx.Err() != nil {
		return Unknown, -1, output, ctx.Err()
	}
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return Unknown, -1, output, err
		}
		code = exit.ExitCode()
	}
	state := StateFromExitCode(code)
	if state == Unknown {
		return state, code, output, fmt.Errorf("official service status returned exit code %d: %s", code, output)
	}
	return state, code, output, nil
}

// Inspect first distinguishes missing/stopped services without attempting the
// relay. SCM is queried with query-only rights, including when the service is
// stopped, so stale service registrations are visible in the report.
func Inspect(ctx context.Context, executable string) (Info, error) {
	return inspect(ctx, executable, checks{cliStatus, readLayout, queryService})
}

type checks struct {
	status  func(context.Context, string) (State, int, string, error)
	layout  func() (Layout, error)
	service func(*Info, string) error
}

func inspect(ctx context.Context, executable string, check checks) (Info, error) {
	info := Info{Name: ServiceName, State: Unknown, CLIExitCode: -1}
	var err error
	info.State, info.CLIExitCode, info.CLIOutput, err = check.status(ctx, executable)
	if err != nil {
		return info, fail("service_status", "query official service status: %w", err)
	}
	if info.State == NotInstalled {
		return info, fail("service_not_installed", "Windows service %s is not installed", ServiceName)
	}
	info.Layout, err = check.layout()
	if err != nil {
		return info, fail("installation_layout", "read 64-bit installation registry: %w", err)
	}
	if err = validateLayout(info.Layout); err != nil {
		return info, err
	}
	if err = check.service(&info, executable); err != nil {
		return info, fail("service_configuration", "query Windows service configuration: %w", err)
	}
	return info, validate(info)
}

func validate(info Info) error {
	if err := validateLayout(info.Layout); err != nil {
		return err
	}
	if !info.PathMatches {
		return fail("service_path", "Windows service points to %q instead of the current installation daemon", info.Executable)
	}
	if !strings.EqualFold(info.Account, "LocalSystem") {
		return fail("service_identity", "Windows service account is %q; expected LocalSystem", info.Account)
	}
	if info.Layout.DaemonDataDirectory != "" && !samePath(info.WorkingDirectory, info.Layout.DaemonDataDirectory) {
		return fail("service_configuration", "service working directory %q differs from installation registry %q", info.WorkingDirectory, info.Layout.DaemonDataDirectory)
	}
	if info.State != Running || info.NativeState != "running" {
		return fail("service_not_running", "Windows service %s is not running (CLI %s, SCM %s)", ServiceName, info.State, info.NativeState)
	}
	return nil
}

func validateLayout(layout Layout) error {
	if !layout.RegistryPresent || !layout.VersionPresent {
		return fail("installation_layout", "installed service has no valid 64-bit installation LayoutVersion; reinstall the application")
	}
	if layout.Version != 2 {
		return fail("installation_layout", "unsupported installation LayoutVersion %d; expected 2", layout.Version)
	}
	return nil
}

func CheckVersion(bundled, running string) error {
	if bundled == "" || running == "" || bundled != running {
		return fail("daemon_version", "running daemon version %q differs from bundled version %q; service repair is required", running, bundled)
	}
	return nil
}

func nativeState(state uint32) string {
	switch state {
	case 1:
		return "stopped"
	case 2:
		return "start_pending"
	case 3:
		return "stop_pending"
	case 4:
		return "running"
	case 5:
		return "continue_pending"
	case 6:
		return "pause_pending"
	case 7:
		return "paused"
	default:
		return "unknown"
	}
}

// Official serviceInstall writes these arguments. The optional development
// flag is only reported here; this package never turns it on.
func serviceArguments(info *Info, args []string) error {
	if len(args) > 0 {
		info.Executable = args[0]
	}
	if (len(args) != 4 && len(args) != 5) || args[1] != "run" || args[2] != "--working-directory" || args[3] == "" {
		return errors.New("service command line does not match the official run --working-directory layout")
	}
	if len(args) == 5 && args[4] != "--allow-unsafe-installation-directory-permissions" {
		return errors.New("service command line has an unexpected extra argument")
	}
	info.WorkingDirectory = args[3]
	return nil
}
