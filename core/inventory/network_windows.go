//go:build windows

package inventory

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

func socketIP(address windows.SocketAddress) string {
	ip := address.IP()
	if ip == nil {
		return ""
	}
	value := ip.String()
	if ip.To4() == nil && address.SockaddrLength >= int32(unsafe.Sizeof(windows.RawSockaddrInet6{})) {
		raw := (*windows.RawSockaddrInet6)(unsafe.Pointer(address.Sockaddr))
		if raw.Scope_id != 0 {
			value += "%" + strconv.FormatUint(uint64(raw.Scope_id), 10)
		}
	}
	return value
}

func sortedUnique(values []string) []string {
	sort.Strings(values)
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" && (len(out) == 0 || out[len(out)-1] != v) {
			out = append(out, v)
		}
	}
	return out
}

func decodeAdapters(ctx context.Context, first *windows.IpAdapterAddresses) ([]AdapterConfig, error) {
	result := []AdapterConfig{}
	for current, count := first, 0; current != nil; current, count = current.Next, count+1 {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if count >= 4096 {
			return result, fmt.Errorf("too many network adapters")
		}
		// Fields through gateways are present in Vista/Win7, but older/short
		// records must not be read as the latest extended structure.
		minimum := unsafe.Offsetof(current.FirstGatewayAddress) + unsafe.Sizeof(current.FirstGatewayAddress)
		if uintptr(current.Length) < minimum {
			return result, fmt.Errorf("short adapter information record")
		}
		info := AdapterConfig{Index: current.IfIndex, IPv6Index: current.Ipv6IfIndex, ID: windows.BytePtrToString(current.AdapterName), Name: windows.UTF16PtrToString(current.FriendlyName), Operational: current.OperStatus == windows.IfOperStatusUp, IPv4Enabled: current.Flags&128 != 0, DHCPv4Enabled: current.Flags&4 != 0, DNSSuffix: windows.UTF16PtrToString(current.DnsSuffix), TransmitBitsPerSecond: current.TransmitLinkSpeed, ReceiveBitsPerSecond: current.ReceiveLinkSpeed}
		for dns, n := current.FirstDnsServerAddress, 0; dns != nil; dns, n = dns.Next, n+1 {
			if n >= 4096 {
				return result, fmt.Errorf("too many DNS addresses")
			}
			info.DNSServers = append(info.DNSServers, socketIP(dns.Address))
		}
		for gateway, n := current.FirstGatewayAddress, 0; gateway != nil; gateway, n = gateway.Next, n+1 {
			if n >= 4096 {
				return result, fmt.Errorf("too many gateway addresses")
			}
			info.Gateways = append(info.Gateways, socketIP(gateway.Address))
		}
		info.DNSServers = sortedUnique(info.DNSServers)
		info.Gateways = sortedUnique(info.Gateways)
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Index == result[j].Index {
			return result[i].ID < result[j].ID
		}
		return result[i].Index < result[j].Index
	})
	return result, nil
}

func (Native) NetworkConfig(ctx context.Context) ([]AdapterConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	size := uint32(15 * 1024)
	for tries := 0; tries < 4; tries++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if size == 0 || size > 4<<20 {
			return nil, fmt.Errorf("adapter buffer exceeds limit")
		}
		buffer := make([]byte, size)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		// INCLUDE_GATEWAYS (0x80), SKIP_ANYCAST (2), SKIP_MULTICAST (4).
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, 0x86, 0, first, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err == windows.ERROR_NO_DATA {
			return []AdapterConfig{}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read adapter configuration failed")
		}
		result, err := decodeAdapters(ctx, first)
		runtime.KeepAlive(buffer)
		return result, err
	}
	return nil, fmt.Errorf("adapter list changed repeatedly")
}
