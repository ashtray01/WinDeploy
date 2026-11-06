package inventory

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func TestInterfaceEnumerationAndPartialErrors(t *testing.T) {
	interfaces := []net.Interface{{Index: 2, Name: "Ethernet", MTU: 1500, Flags: net.FlagUp}, {Index: 1, Name: "Loopback", Flags: net.FlagLoopback}}
	got, err := readInterfaces(context.Background(), func() ([]net.Interface, error) { return interfaces, nil }, func(iface net.Interface) ([]net.Addr, error) {
		if iface.Index == 1 {
			return nil, errors.New("private address error")
		}
		_, v4, _ := net.ParseCIDR("192.0.2.10/24")
		_, v6, _ := net.ParseCIDR("2001:db8::1/64")
		return []net.Addr{v4, v6}, nil
	})
	if err == nil || strings.Contains(err.Error(), "private") || len(got) != 2 || got[0].Index != 1 || !got[0].Loopback || got[1].Issue != "" || len(got[1].Addresses) != 2 || !got[1].Up {
		t.Fatalf("%+v %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readInterfaces(ctx, func() ([]net.Interface, error) { t.Fatal("called after cancellation"); return nil, nil }, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := readInterfaces(context.Background(), func() ([]net.Interface, error) { return nil, errors.New("private") }, nil); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatalf("%v", err)
	}
}

func TestNativeInterfacesReadOnly(t *testing.T) {
	interfaces, err := (Native{}).Interfaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(interfaces) == 0 {
		t.Fatal("no interfaces")
	}
}
