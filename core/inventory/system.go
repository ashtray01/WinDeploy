// Package inventory reads workstation facts without changing system settings.
package inventory

import (
	"context"
	"errors"
	"fmt"
)

var ErrUnsupported = errors.New("inventory provider is unsupported on this platform")

// Native is the read-only Windows provider. Methods check context around native
// calls; an in-flight synchronous Windows API call cannot be forcibly cancelled.
type Native struct{}

type Version struct {
	Major            uint32 `json:"major"`
	Minor            uint32 `json:"minor"`
	Build            uint32 `json:"build"`
	ServicePackMajor uint16 `json:"service_pack_major"`
	ProductType      uint8  `json:"product_type"`
}

// Token uses the profile vocabulary and deliberately excludes Server, Win7 RTM
// and unrecognized NT versions. A version token is not an API capability probe.
func (v Version) Token() string {
	if v.ProductType != 1 {
		return ""
	}
	switch {
	case v.Major == 6 && v.Minor == 1 && v.ServicePackMajor >= 1:
		return "windows-7-sp1"
	case v.Major == 10 && v.Minor == 0 && v.Build >= 22000:
		return "windows-11"
	case v.Major == 10 && v.Minor == 0 && v.Build >= 10240:
		return "windows-10"
	default:
		return ""
	}
}

func (v Version) Label() string {
	return fmt.Sprintf("Windows NT %d.%d build %d SP%d (product type %d)", v.Major, v.Minor, v.Build, v.ServicePackMajor, v.ProductType)
}

type SystemInfo struct {
	Hostname            string  `json:"hostname"`
	Architecture        string  `json:"architecture"`
	ProcessArchitecture string  `json:"process_architecture"`
	Windows             Version `json:"windows"`
}

type SystemProvider interface {
	System(context.Context) (SystemInfo, error)
}

func architecture(code uint16) string {
	switch code {
	case 0:
		return "386"
	case 9:
		return "amd64"
	case 12:
		return "arm64"
	default:
		return "unknown"
	}
}
