package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/mgr"
)

func hideWindow(cmd *exec.Cmd)  { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }

func readLayout() (Layout, error) {
	var layout Layout
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\SagerNet\sing-box`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if errors.Is(err, registry.ErrNotExist) {
		return layout, nil
	}
	if err != nil {
		return layout, err
	}
	defer key.Close()
	layout.RegistryPresent = true
	var valueType uint32
	layout.Version, valueType, err = key.GetIntegerValue("LayoutVersion")
	if err == nil {
		if valueType != registry.DWORD {
			return layout, errors.New("LayoutVersion must be a DWORD registry value")
		}
		layout.VersionPresent = true
	} else if !errors.Is(err, registry.ErrNotExist) {
		return layout, err
	}
	for name, dest := range map[string]*string{"ApplicationDataDirectory": &layout.ApplicationDataDirectory, "DaemonDataDirectory": &layout.DaemonDataDirectory} {
		value, _, err := key.GetStringValue(name)
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			return layout, fmt.Errorf("%s: %w", name, err)
		}
		*dest = value
	}
	return layout, nil
}

func queryService(info *Info, expected string) error {
	manager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(manager)
	name, err := windows.UTF16PtrFromString(ServiceName)
	if err != nil {
		return err
	}
	handle, err := windows.OpenService(manager, name, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return err
	}
	service := &mgr.Service{Name: ServiceName, Handle: handle}
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return err
	}
	info.ProcessID = status.ProcessId
	info.NativeState = nativeState(uint32(status.State))
	config, err := service.Config()
	if err != nil {
		return err
	}
	info.CommandLine, info.Account = config.BinaryPathName, config.ServiceStartName
	args, err := windows.DecomposeCommandLine(config.BinaryPathName)
	if err != nil {
		return err
	}
	if err = serviceArguments(info, args); err != nil {
		return err
	}
	if !samePath(info.Executable, expected) {
		return nil
	}
	actualFile, err := os.Stat(info.Executable)
	if err != nil {
		return fmt.Errorf("service executable: %w", err)
	}
	expectedFile, err := os.Stat(expected)
	if err != nil {
		return fmt.Errorf("bundled daemon: %w", err)
	}
	info.PathMatches = os.SameFile(actualFile, expectedFile)
	return nil
}
