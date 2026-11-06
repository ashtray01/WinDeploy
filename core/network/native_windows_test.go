//go:build windows

package network

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"windeploy/core/inventory"
)

// TestNativeDiagnostics runs the actual Windows adapters: system DNS, TCP to a
// local listener, HTTP to a local server and ICMP through ping.exe. It does not
// touch remote infrastructure.
func TestNativeDiagnostics(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			connection.Close()
		}
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	checks := []Check{
		{ID: "dns-localhost", Kind: KindDNS, DNSName: "localhost", Timeout: 5 * time.Second, Required: true},
		{ID: "tcp-local", Kind: KindTCP, Host: "127.0.0.1", Port: port, Timeout: 5 * time.Second, Required: true},
		{ID: "http-local", Kind: KindHTTP, URL: server.URL, Timeout: 5 * time.Second, Required: true},
		{ID: "icmp-loopback", Kind: KindICMP, Host: "127.0.0.1", Timeout: 5 * time.Second},
	}
	report, err := Diagnose(context.Background(), checks, NativeDependencies())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Complete {
		t.Fatal("native diagnostics did not complete")
	}
	statuses := map[string]Status{}
	for _, result := range report.Results {
		statuses[result.ID] = result.Status
	}
	for _, id := range []string{"dns-localhost", "tcp-local", "http-local"} {
		if statuses[id] != StatusOK {
			t.Errorf("%s status = %s", id, statuses[id])
		}
	}
	if statuses["icmp-loopback"] != StatusOK && statuses["icmp-loopback"] != StatusFailed {
		// ICMP may be blocked by host policy; a truthful failure is acceptable.
		t.Errorf("unexpected ICMP status: %s", statuses["icmp-loopback"])
	}
	adapter := inventory.AdapterConfig{Index: 1, Name: "loopback", Operational: true, IPv4Enabled: true, DHCPv4Enabled: false}
	dhcpStatus, _, _ := checkDHCP(context.Background(), Check{Adapter: &adapter})
	if dhcpStatus != StatusOK {
		t.Fatalf("static adapter DHCP status = %s", dhcpStatus)
	}
}
