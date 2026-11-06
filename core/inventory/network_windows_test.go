//go:build windows

package inventory

import (
	"context"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDecodeAdapterConfiguration(t *testing.T) {
	name, _ := windows.UTF16PtrFromString("Ethernet")
	ip := windows.RawSockaddrInet4{Family: windows.AF_INET, Addr: [4]byte{192, 0, 2, 1}}
	address := windows.SocketAddress{Sockaddr: (*syscall.RawSockaddrAny)(unsafe.Pointer(&ip)), SockaddrLength: int32(unsafe.Sizeof(ip))}
	dns := windows.IpAdapterDnsServerAdapter{Address: address}
	gateway := windows.IpAdapterGatewayAddress{Address: address}
	adapter := windows.IpAdapterAddresses{Length: uint32(unsafe.Sizeof(windows.IpAdapterAddresses{})), IfIndex: 7, FriendlyName: name, Flags: 132, OperStatus: windows.IfOperStatusUp, FirstDnsServerAddress: &dns, FirstGatewayAddress: &gateway, TransmitLinkSpeed: 1_000_000_000}
	got, err := decodeAdapters(context.Background(), &adapter)
	if err != nil || len(got) != 1 || !got[0].DHCPv4Enabled || !got[0].IPv4Enabled || len(got[0].Gateways) != 1 || got[0].DNSServers[0] != "192.0.2.1" || got[0].TransmitBitsPerSecond != 1_000_000_000 {
		t.Fatalf("%+v %v", got, err)
	}
	adapter.Length = 8
	if _, err = decodeAdapters(context.Background(), &adapter); err == nil {
		t.Fatal("short record accepted")
	}
	ipv6 := windows.RawSockaddrInet6{Family: windows.AF_INET6, Addr: [16]byte{0xfe, 0x80}, Scope_id: 7}
	value := socketIP(windows.SocketAddress{Sockaddr: (*syscall.RawSockaddrAny)(unsafe.Pointer(&ipv6)), SockaddrLength: int32(unsafe.Sizeof(ipv6))})
	if value != "fe80::%7" {
		t.Fatalf("IPv6 scope lost: %s", value)
	}
}

func TestNativeNetworkConfigurationReadOnly(t *testing.T) {
	_, err := (Native{}).NetworkConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}
