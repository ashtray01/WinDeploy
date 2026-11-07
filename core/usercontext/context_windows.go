//go:build windows

package usercontext

import (
	"context"
	"golang.org/x/sys/windows"
)

// Read compares the process owner with the shell on this session/desktop.
// No shell, denied token access or a disappearing shell fails closed. This does
// not impersonate anyone or request elevation/SeDebugPrivilege.
func (Native) Read(ctx context.Context) (Identity, error) {
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	processUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return Identity{}, err
	}
	user32 := windows.NewLazySystemDLL("user32.dll")
	for _, name := range []string{"GetShellWindow", "GetWindowThreadProcessId"} {
		if err := user32.NewProc(name).Find(); err != nil {
			return Identity{}, ErrUnconfirmed
		}
	}
	shell := windows.GetShellWindow()
	if shell == 0 {
		return Identity{}, ErrUnconfirmed
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(shell, &pid); err != nil || pid == 0 {
		return Identity{}, ErrUnconfirmed
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return Identity{}, err
	}
	defer windows.CloseHandle(process)
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return Identity{}, err
	}
	defer token.Close()
	shellUser, err := token.GetTokenUser()
	if err != nil {
		return Identity{}, err
	}
	// Recheck the shell handle after reading its owner to reject shell replacement.
	if windows.GetShellWindow() != shell {
		return Identity{}, ErrUnconfirmed
	}
	var finalPID uint32
	if _, err := windows.GetWindowThreadProcessId(shell, &finalPID); err != nil || finalPID != pid {
		return Identity{}, ErrUnconfirmed
	}
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	return Identity{ProcessSID: processUser.User.Sid.String(), ShellSID: shellUser.User.Sid.String()}, nil
}
