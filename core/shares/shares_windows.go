//go:build windows

package shares

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WindowsCredentialStore uses the Windows Credential Manager (DPAPI-protected).
type WindowsCredentialStore struct{}

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	errorNotFound           = 1168
)

type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         *byte
	TargetAlias        *uint16
	UserName           *uint16
}

var (
	advapi32          = windows.NewLazySystemDLL("advapi32.dll")
	procCredWriteW    = advapi32.NewProc("CredWriteW")
	procCredReadW     = advapi32.NewProc("CredReadW")
	procCredDeleteW   = advapi32.NewProc("CredDeleteW")
	procCredFree      = advapi32.NewProc("CredFree")
	mpr               = windows.NewLazySystemDLL("mpr.dll")
	procWNetAddConn2W = mpr.NewProc("WNetAddConnection2W")
	procWNetCancel2W  = mpr.NewProc("WNetCancelConnection2W")
	procWNetOpenEnum  = mpr.NewProc("WNetOpenEnumW")
	procWNetEnum      = mpr.NewProc("WNetEnumResourceW")
	procWNetCloseEnum = mpr.NewProc("WNetCloseEnum")
	ole32             = windows.NewLazySystemDLL("ole32.dll")
	procCoCreate      = ole32.NewProc("CoCreateInstance")
)

// Save implements CredentialStore.
func (WindowsCredentialStore) Save(ctx context.Context, target, username, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("invalid credential target")
	}
	userPtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return fmt.Errorf("invalid user name")
	}
	passwordPtr, err := windows.UTF16FromString(password)
	if err != nil {
		return fmt.Errorf("invalid password")
	}
	// UTF16FromString appends a NUL; the credential blob excludes it.
	blobSize := len(passwordPtr) - 1
	if blobSize < 0 {
		blobSize = 0
	}
	credential := credentialW{
		Type:               credTypeGeneric,
		TargetName:         targetPtr,
		CredentialBlobSize: uint32(blobSize * 2),
		CredentialBlob:     (*byte)(unsafe.Pointer(&passwordPtr[0])),
		Persist:            credPersistLocalMachine,
		UserName:           userPtr,
	}
	runtime.KeepAlive(passwordPtr)
	runtime.KeepAlive(targetPtr)
	runtime.KeepAlive(userPtr)
	result, _, callErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&credential)), 0)
	if result == 0 {
		return fmt.Errorf("credential could not be saved (%v)", callErr)
	}
	return nil
}

// Read implements CredentialStore.
func (WindowsCredentialStore) Read(ctx context.Context, target string) (Credential, bool, error) {
	if err := ctx.Err(); err != nil {
		return Credential{}, false, err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return Credential{}, false, fmt.Errorf("invalid credential target")
	}
	var credential *credentialW
	result, _, callErr := procCredReadW.Call(uintptr(unsafe.Pointer(targetPtr)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&credential)))
	if result == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorNotFound {
			return Credential{}, false, nil
		}
		return Credential{}, false, fmt.Errorf("credential could not be read")
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(credential)))
	password := ""
	if credential.CredentialBlob != nil && credential.CredentialBlobSize > 0 {
		bytes := unsafe.Slice(credential.CredentialBlob, credential.CredentialBlobSize)
		units := make([]uint16, credential.CredentialBlobSize/2)
		for i := range units {
			units[i] = uint16(bytes[i*2]) | uint16(bytes[i*2+1])<<8
		}
		password = windows.UTF16ToString(units)
	}
	return Credential{Username: windows.UTF16PtrToString(credential.UserName), Password: password}, true, nil
}

// Delete implements CredentialStore.
func (WindowsCredentialStore) Delete(ctx context.Context, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return fmt.Errorf("invalid credential target")
	}
	result, _, callErr := procCredDeleteW.Call(uintptr(unsafe.Pointer(targetPtr)), credTypeGeneric, 0)
	if result == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorNotFound {
			return nil
		}
		return fmt.Errorf("credential could not be deleted")
	}
	return nil
}

// WindowsConnector maps UNC resources through the WNet API.
type WindowsConnector struct{}

type netResourceW struct {
	Scope       uint32
	Type        uint32
	DisplayType uint32
	Usage       uint32
	LocalName   *uint16
	RemoteName  *uint16
	Comment     *uint16
	Provider    *uint16
}

const (
	resourceConnected    = 1
	resourceTypeDisk     = 1
	connectUpdateProfile = 1
	errorNoMoreItems     = 259
	errorSessionConflict = 1219
)

// Connect implements Connector. The password is passed in memory to WNet and is
// never written to a command line or log.
func (WindowsConnector) Connect(ctx context.Context, request ConnectionRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	remotePtr, err := windows.UTF16PtrFromString(request.RemoteName)
	if err != nil {
		return fmt.Errorf("invalid UNC path")
	}
	resource := netResourceW{Type: resourceTypeDisk, RemoteName: remotePtr}
	var userPtr, passwordPtr *uint16
	if request.Username != "" {
		if userPtr, err = windows.UTF16PtrFromString(request.Username); err != nil {
			return fmt.Errorf("invalid user name")
		}
	}
	if request.Password != "" {
		if passwordPtr, err = windows.UTF16PtrFromString(request.Password); err != nil {
			return fmt.Errorf("invalid password")
		}
	}
	flags := uintptr(0)
	if request.Persistent {
		flags = connectUpdateProfile
	}
	code, _, _ := procWNetAddConn2W.Call(uintptr(unsafe.Pointer(&resource)), uintptr(unsafe.Pointer(passwordPtr)), uintptr(unsafe.Pointer(userPtr)), flags)
	if code != 0 {
		if syscall.Errno(code) == errorSessionConflict {
			return ErrCredentialConflict
		}
		return fmt.Errorf("network connection failed (code %d)", code)
	}
	return nil
}

// List implements Connector.
func (WindowsConnector) List(ctx context.Context) ([]Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var handle windows.Handle
	code, _, _ := procWNetOpenEnum.Call(resourceConnected, resourceTypeDisk, 0, 0, uintptr(unsafe.Pointer(&handle)))
	if code != 0 {
		return nil, fmt.Errorf("existing connections are unavailable (code %d)", code)
	}
	defer procWNetCloseEnum.Call(uintptr(handle))
	connections := []Connection{}
	buffer := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return connections, err
		}
		count := uint32(1)
		size := uint32(len(buffer))
		code, _, _ = procWNetEnum.Call(uintptr(handle), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
		if syscall.Errno(code) == errorNoMoreItems {
			break
		}
		if code != 0 {
			return connections, fmt.Errorf("existing connections are unavailable (code %d)", code)
		}
		if count == 0 {
			break
		}
		entries := unsafe.Slice((*netResourceW)(unsafe.Pointer(&buffer[0])), int(count))
		for _, entry := range entries {
			connections = append(connections, Connection{RemoteName: windows.UTF16PtrToString(entry.RemoteName), LocalName: windows.UTF16PtrToString(entry.LocalName)})
		}
		if len(connections) > 512 {
			return connections, fmt.Errorf("too many existing connections")
		}
	}
	return connections, nil
}

// Disconnect implements Connector.
func (WindowsConnector) Disconnect(ctx context.Context, remoteName string, force bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	namePtr, err := windows.UTF16PtrFromString(remoteName)
	if err != nil {
		return fmt.Errorf("invalid connection name")
	}
	forceFlag := uintptr(0)
	if force {
		forceFlag = 1
	}
	code, _, _ := procWNetCancel2W.Call(uintptr(unsafe.Pointer(namePtr)), 0, forceFlag)
	if code != 0 {
		return fmt.Errorf("network connection could not be removed (code %d)", code)
	}
	return nil
}

// WindowsShortcut creates .lnk files through the native IShellLink COM interface.
type WindowsShortcut struct{}

// DesktopDirectory returns the real desktop folder, including OneDrive redirection.
func DesktopDirectory() (string, error) {
	path, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, windows.KF_FLAG_DEFAULT)
	if err != nil || path == "" {
		if home, homeErr := os.UserHomeDir(); homeErr == nil {
			path = filepath.Join(home, "Desktop")
		} else {
			return "", fmt.Errorf("desktop directory is unavailable")
		}
	}
	return path, nil
}

var (
	clsidShellLink  = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIShellLinkW  = windows.GUID{Data1: 0x000214F9, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIPersistFile = windows.GUID{Data1: 0x0000010B, Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

const (
	clsctxInprocServer = 0x1
	coinitApartment    = 0x2
)

// Create implements ShortcutCreator.
func (WindowsShortcut) Create(ctx context.Context, spec ShortcutSpec) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if spec.Path == "" || spec.Target == "" {
		return fmt.Errorf("shortcut path and target are required")
	}
	if dir := filepath.Dir(spec.Path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("shortcut directory is unavailable")
		}
	}
	initialized := false
	if err := windows.CoInitializeEx(0, coinitApartment); err == nil {
		initialized = true
	}
	if initialized {
		defer windows.CoUninitialize()
	}
	var link unsafe.Pointer
	hr, _, _ := procCoCreate.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIShellLinkW)), uintptr(unsafe.Pointer(&link)))
	if failedHR(hr) || link == nil {
		return fmt.Errorf("shell link could not be created")
	}
	defer comCall(link, 2)
	targetPtr, err := windows.UTF16PtrFromString(spec.Target)
	if err != nil {
		return fmt.Errorf("invalid shortcut target")
	}
	if failedHR(comCall(link, 20, uintptr(unsafe.Pointer(targetPtr)))) {
		return fmt.Errorf("shortcut target could not be set")
	}
	if description := spec.Description; description != "" {
		if descPtr, descErr := windows.UTF16PtrFromString(description); descErr == nil {
			_ = comCall(link, 7, uintptr(unsafe.Pointer(descPtr)))
		}
	}
	var persist unsafe.Pointer
	if failedHR(comCall(link, 0, uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&persist)))) || persist == nil {
		return fmt.Errorf("shortcut could not be persisted")
	}
	defer comCall(persist, 2)
	pathPtr, err := windows.UTF16PtrFromString(spec.Path)
	if err != nil {
		return fmt.Errorf("invalid shortcut path")
	}
	if failedHR(comCall(persist, 6, uintptr(unsafe.Pointer(pathPtr)), 1)) {
		return fmt.Errorf("shortcut file could not be written")
	}
	return nil
}

// comCall invokes a COM vtable method.
func comCall(object unsafe.Pointer, method int, args ...uintptr) uintptr {
	vtable := *(**[32]uintptr)(object)
	fn := vtable[method]
	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, uintptr(object))
	callArgs = append(callArgs, args...)
	result, _, _ := syscall.SyscallN(fn, callArgs...)
	return result
}

func failedHR(hr uintptr) bool {
	return int32(uint32(hr)) < 0
}
