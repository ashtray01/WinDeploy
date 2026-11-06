package inventory

import (
	"context"
	"fmt"
	"net"
	"sort"
)

type InterfaceInfo struct {
	Index           int      `json:"index"`
	Name            string   `json:"name"`
	HardwareAddress string   `json:"hardware_address,omitempty"`
	MTU             int      `json:"mtu"`
	Up              bool     `json:"up"`
	Loopback        bool     `json:"loopback"`
	Addresses       []string `json:"addresses"`
	Issue           string   `json:"issue,omitempty"`
}
type InterfaceProvider interface {
	Interfaces(context.Context) ([]InterfaceInfo, error)
}

// Interfaces reports IPv4/IPv6 prefixes and local flags, not reachability.
func (Native) Interfaces(ctx context.Context) ([]InterfaceInfo, error) {
	return readInterfaces(ctx, net.Interfaces, func(iface net.Interface) ([]net.Addr, error) { return iface.Addrs() })
}

func readInterfaces(ctx context.Context, list func() ([]net.Interface, error), addresses func(net.Interface) ([]net.Addr, error)) ([]InterfaceInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	interfaces, err := list()
	if err != nil {
		return nil, fmt.Errorf("enumerate interfaces failed")
	}
	result := make([]InterfaceInfo, 0, len(interfaces))
	failed := false
	for _, iface := range interfaces {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		info := InterfaceInfo{Index: iface.Index, Name: iface.Name, HardwareAddress: iface.HardwareAddr.String(), MTU: iface.MTU, Up: iface.Flags&net.FlagUp != 0, Loopback: iface.Flags&net.FlagLoopback != 0, Addresses: []string{}}
		addrs, readErr := addresses(iface)
		if readErr != nil {
			info.Issue = "read interface addresses failed"
			failed = true
		} else {
			for _, addr := range addrs {
				if addr == nil {
					info.Issue = "invalid interface address"
					failed = true
					continue
				}
				if _, _, err := net.ParseCIDR(addr.String()); err != nil {
					info.Issue = "invalid interface address"
					failed = true
					continue
				}
				info.Addresses = append(info.Addresses, addr.String())
			}
		}
		sort.Strings(info.Addresses)
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Index == result[j].Index {
			return result[i].Name < result[j].Name
		}
		return result[i].Index < result[j].Index
	})
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if failed {
		return result, fmt.Errorf("some interface addresses are unavailable")
	}
	return result, nil
}
