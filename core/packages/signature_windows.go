//go:build windows

package packages

import (
	"context"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CMSG_SIGNER_INFO_PARAM is the CryptMsgGetParam selector for the signer info.
const cmsgSignerInfoParam = 6

type cryptAttribute struct {
	OID   *byte
	Value windows.DataBlob
}

type cryptAttributes struct {
	Count      uint32
	Attributes *cryptAttribute
}

// cryptSignerInfo mirrors CMSG_SIGNER_INFO from wincrypt.h.
type cryptSignerInfo struct {
	Version                 uint32
	Issuer                  windows.CertNameBlob
	SerialNumber            windows.CryptIntegerBlob
	HashAlgorithm           windows.CryptAlgorithmIdentifier
	HashEncryptionAlgorithm windows.CryptAlgorithmIdentifier
	EncryptedHash           windows.CryptDataBlob
	AuthAttrs               cryptAttributes
	UnauthAttrs             cryptAttributes
}

var (
	crypt32DLL        = windows.NewLazySystemDLL("crypt32.dll")
	procCryptMsgClose = crypt32DLL.NewProc("CryptMsgClose")
)

// WindowsSigner verifies Authenticode signatures through WinVerifyTrust and reads
// the signer certificate subject from the embedded PKCS#7 store. Revocation
// checking is disabled so an offline machine can still verify trust; the root
// certificate chain is still validated by Windows.
type WindowsSigner struct{}

// Verify implements Signer. An unsigned or untrusted binary returns
// Verified=false without a transport error.
func (WindowsSigner) Verify(ctx context.Context, path string) (Signature, error) {
	if err := ctx.Err(); err != nil {
		return Signature{}, err
	}
	trusted, verifyErr := winVerifyTrust(path)
	signature := Signature{Verified: trusted}
	subject, subjectErr := signerSubject(path)
	if subjectErr == nil && subject != "" {
		signature.Subject = subject
		signature.Signer = commonName(subject)
	}
	if verifyErr != nil && !trusted {
		// WinVerifyTrust reports every untrusted state as an error. Treat it as a
		// negative signature result rather than an infrastructure failure.
		return signature, nil
	}
	if err := ctx.Err(); err != nil {
		return Signature{}, err
	}
	return signature, nil
}

func winVerifyTrust(path string) (bool, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, fmt.Errorf("invalid path")
	}
	fileInfo := windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: path16,
	}
	data := windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_NONE,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&fileInfo),
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
	}
	verifyErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	// Always close Wintrust state, even after a failed verification.
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	return verifyErr == nil, verifyErr
}

func signerSubject(path string) (string, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	var (
		encoding   uint32
		contentTyp uint32
		formatTyp  uint32
		store      windows.Handle
		message    windows.Handle
	)
	if err := windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(path16),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		windows.CERT_QUERY_FORMAT_FLAG_BINARY,
		0,
		&encoding, &contentTyp, &formatTyp, &store, &message, nil,
	); err != nil {
		return "", fmt.Errorf("signature is not available")
	}
	if store != 0 {
		defer windows.CertCloseStore(store, 0)
	}
	if message == 0 {
		return "", fmt.Errorf("signature message is missing")
	}
	defer procCryptMsgClose.Call(uintptr(message))

	var size uint32
	result, _, callErr := procCryptMsgGetParam.Call(uintptr(message), cmsgSignerInfoParam, 0, 0, uintptr(unsafe.Pointer(&size)))
	if result == 0 || size == 0 || size > 1<<20 {
		return "", fmt.Errorf("signer information is unavailable: %v", callErr)
	}
	buffer := make([]byte, size)
	result, _, callErr = procCryptMsgGetParam.Call(uintptr(message), cmsgSignerInfoParam, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if result == 0 {
		return "", fmt.Errorf("signer information is unavailable: %v", callErr)
	}
	signer := (*cryptSignerInfo)(unsafe.Pointer(&buffer[0]))
	info := windows.CertInfo{Issuer: signer.Issuer, SerialNumber: signer.SerialNumber}
	context, err := windows.CertFindCertificateInStore(store, encoding, 0, windows.CERT_FIND_SUBJECT_CERT, unsafe.Pointer(&info), nil)
	if err != nil || context == nil {
		return "", fmt.Errorf("signer certificate is unavailable")
	}
	defer windows.CertFreeCertificateContext(context)
	chars := windows.CertGetNameString(context, windows.CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nil, nil, 0)
	if chars <= 1 || chars > 512 {
		return "", fmt.Errorf("signer name is unavailable")
	}
	name := make([]uint16, chars)
	if windows.CertGetNameString(context, windows.CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nil, &name[0], chars) == 0 {
		return "", fmt.Errorf("signer name is unavailable")
	}
	return windows.UTF16ToString(name), nil
}

// commonName extracts CN=... from a certificate display name when present.
func commonName(subject string) string {
	lower := subject
	for _, part := range splitSubject(lower) {
		trimmed := trimSpace(part)
		if len(trimmed) > 3 && (trimmed[:3] == "CN=" || trimmed[:3] == "cn=") {
			return trimSpace(trimmed[3:])
		}
	}
	return subject
}

func splitSubject(value string) []string {
	parts := []string{}
	start := 0
	for i := 0; i < len(value); i++ {
		if value[i] == ',' {
			parts = append(parts, value[start:i])
			start = i + 1
		}
	}
	parts = append(parts, value[start:])
	return parts
}

func trimSpace(value string) string {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}

var procCryptMsgGetParam = func() *windows.LazyProc {
	dll := windows.NewLazySystemDLL("crypt32.dll")
	return dll.NewProc("CryptMsgGetParam")
}()
