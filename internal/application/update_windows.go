//go:build windows

package application

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// relaunchAfter opens the application again once the installer, which the
// core runs as the service account, has exited. The installer replaces this
// executable, so a detached PowerShell in this user's session waits for it.
func relaunchAfter(installerName string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := "Start-Sleep 3; $deadline=(Get-Date).AddMinutes(10); " +
		"while ((Get-Process -Name " + quote(installerName) + " -ErrorAction SilentlyContinue) -and (Get-Date) -lt $deadline) { Start-Sleep 1 }; " +
		"Start-Process -FilePath " + quote(exe)
	// The same arguments, as a development data directory, open it again.
	// Start-Process joins an argument array without quoting, so pass one
	// command line.
	var args []string
	for _, arg := range os.Args[1:] {
		args = append(args, syscall.EscapeArg(arg))
	}
	if len(args) > 0 {
		script += " -ArgumentList " + quote(strings.Join(args, " "))
	}
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000 | 0x00000200} // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP; PowerShell does not run DETACHED_PROCESS
	return command.Start()
}
