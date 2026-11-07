//go:build windows

package trust

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	aceTypeAccessAllowed       = 0x00
	aceTypeAccessDenied        = 0x01
	aceTypeAccessAllowedObject = 0x05
	aceTypeAccessDeniedObject  = 0x06
	aceFlagInheritOnly         = 0x08
	fileDeleteChild            = 0x00000040
)

// writeAccess covers every way a trustee could rewrite the object itself.
// Do not use FILE_GENERIC_WRITE here: that composite mask also contains
// READ_CONTROL and SYNCHRONIZE, which are present in an ordinary FILE_GENERIC_READ
// grant. Treating any overlap as write access rejects harmless inherited (RX)
// entries such as endpoint-management and sandbox reader groups.
const writeAccess = uint32(windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA |
	windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES | windows.GENERIC_WRITE |
	windows.GENERIC_ALL | windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER |
	fileDeleteChild)

type aclHeader struct {
	Revision uint8
	Sbz1     uint8
	Size     uint16
	Count    uint16
	Sbz2     uint16
}

type aceHeader struct {
	Type  uint8
	Flags uint8
	Size  uint16
}

// check requires a regular file (or directory) that is not a link, is owned by the
// current process user (or the built-in Administrators group used as the default
// owner by an elevated token) and has a non-null DACL whose only write grants are
// the current user, SYSTEM, Administrators or owner-mapped CREATOR OWNER entries.
// Administrators can take ownership of any local object, so this rejects ordinary
// users and groups rather than machine administrators.
func check(path string, dir bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || info.IsDir() != dir {
		return fmt.Errorf("unexpected path type")
	}
	if dir && isReparsePoint(path) {
		return fmt.Errorf("reparse points are not trusted storage")
	}
	descriptor, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := descriptor.Owner()
	if err != nil || owner == nil || !owner.IsValid() {
		return fmt.Errorf("owner is unavailable")
	}
	processUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || processUser.User.Sid == nil {
		return fmt.Errorf("current user is unavailable")
	}
	administrators, administratorsErr := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if !owner.Equals(processUser.User.Sid) && (administratorsErr != nil || !owner.Equals(administrators)) {
		return fmt.Errorf("owner does not match the current user")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if dacl == nil {
		return fmt.Errorf("missing DACL grants everyone access")
	}
	err = checkWriteGrants((*aclHeader)(unsafe.Pointer(dacl)), processUser.User.Sid)
	// The DACL points into the Go-heap copy owned by descriptor.
	runtime.KeepAlive(descriptor)
	return err
}

func isReparsePoint(path string) bool {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	attributes, err := windows.GetFileAttributes(ptr)
	if err != nil {
		return true
	}
	return attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func checkWriteGrants(dacl *aclHeader, user *windows.SID) error {
	trusted := trustedSIDs(user)
	headerSize := int(unsafe.Sizeof(aclHeader{}))
	remaining := int(dacl.Size) - headerSize
	if remaining < 0 {
		return fmt.Errorf("malformed ACL")
	}
	base := unsafe.Add(unsafe.Pointer(dacl), headerSize)
	offset := 0
	for index := 0; index < int(dacl.Count); index++ {
		if offset+int(unsafe.Sizeof(aceHeader{})) > remaining {
			return fmt.Errorf("malformed ACL")
		}
		header := (*aceHeader)(unsafe.Add(base, offset))
		size := int(header.Size)
		if size < 8 || offset+size > remaining {
			return fmt.Errorf("malformed ACL")
		}
		inheritableOnly := header.Flags&aceFlagInheritOnly != 0
		denied := header.Type == aceTypeAccessDenied || header.Type == aceTypeAccessDeniedObject
		mask := *(*uint32)(unsafe.Add(base, offset+4))
		if !denied && !inheritableOnly && mask&writeAccess != 0 {
			if header.Type != aceTypeAccessAllowed {
				// Object ACEs are not produced by the session layout this package
				// supports; fail closed instead of parsing them partially.
				return fmt.Errorf("unsupported access control entry")
			}
			sid := (*windows.SID)(unsafe.Add(base, offset+8))
			if !withinACE(size, sid) || !trusted(sid) {
				return fmt.Errorf("storage is writable by another principal")
			}
		}
		offset += size
	}
	return nil
}

func withinACE(size int, sid *windows.SID) bool {
	if sid == nil || !sid.IsValid() {
		return false
	}
	// 8 bytes of ACE header and mask, 8 bytes of SID header, then subauthorities.
	return 8+8+4*int(sid.SubAuthorityCount()) <= size
}

func trustedSIDs(user *windows.SID) func(*windows.SID) bool {
	allowed := []*windows.SID{user}
	for _, kind := range []windows.WELL_KNOWN_SID_TYPE{
		windows.WinLocalSystemSid,
		windows.WinBuiltinAdministratorsSid,
		windows.WinCreatorOwnerSid,
		windows.WinCreatorOwnerRightsSid,
	} {
		if sid, err := windows.CreateWellKnownSid(kind); err == nil {
			allowed = append(allowed, sid)
		}
	}
	return func(candidate *windows.SID) bool {
		for _, known := range allowed {
			if known.Equals(candidate) {
				return true
			}
		}
		return false
	}
}
