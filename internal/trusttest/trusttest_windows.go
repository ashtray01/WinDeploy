//go:build windows

package trusttest

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// restrict replaces the directory DACL with full control for the current user,
// SYSTEM and Administrators only, and stops inheritance so sandbox or share ACLs
// cannot keep write access for other principals.
func restrict(dir string) error {
	processUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || processUser.User.Sid == nil {
		return fmt.Errorf("current user is unavailable")
	}
	trustees := []*windows.SID{processUser.User.Sid}
	for _, kind := range []windows.WELL_KNOWN_SID_TYPE{windows.WinLocalSystemSid, windows.WinBuiltinAdministratorsSid} {
		sid, err := windows.CreateWellKnownSid(kind)
		if err != nil {
			return err
		}
		trustees = append(trustees, sid)
	}
	entries := make([]windows.EXPLICIT_ACCESS, 0, len(trustees))
	for _, sid := range trustees {
		entries = append(entries, windows.EXPLICIT_ACCESS{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.SET_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		})
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil)
}
