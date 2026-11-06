package network

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"windeploy/core/inventory"
)

type fakeResolver struct {
	hosts map[string][]string
	err   error
	delay time.Duration
	calls int
}

func (r *fakeResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	r.calls++
	if r.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(r.delay):
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	if hosts, ok := r.hosts[host]; ok {
		return hosts, nil
	}
	return nil, errors.New("not found")
}

type fakeDialer struct {
	refuse map[string]bool
}

func (d *fakeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.refuse[address] {
		return nil, errors.New("connection refused")
	}
	client, server := net.Pipe()
	server.Close()
	return client, nil
}

type fakePinger struct {
	reachable map[string]bool
}

func (p *fakePinger) Ping(ctx context.Context, host string) (time.Duration, error) {
	if p.reachable[host] {
		return 7 * time.Millisecond, nil
	}
	return 0, errors.New("unreachable")
}

func TestBuildChecksIsStableAndDeduplicated(t *testing.T) {
	adapter := inventory.AdapterConfig{Index: 2, Name: "Ethernet", Operational: true, IPv4Enabled: true, DHCPv4Enabled: true, Gateways: []string{"192.168.1.1"}, DNSServers: []string{"192.168.1.1"}}
	plan := Plan{
		Adapters:   []inventory.AdapterConfig{adapter},
		DNSNames:   []string{"example.test", "example.test", " "},
		Servers:    []string{"192.168.1.1"},
		Gateways:   []string{"192.168.1.1"},
		TCPTargets: []Endpoint{{Host: "server.test", Port: 445}, {Host: "bad", Port: 0}},
		ICMPHosts:  []string{"8.8.8.8"},
		HTTPURLs:   []string{"https://example.test/"},
		SMBHosts:   []string{"fileserver"},
	}
	checks := BuildChecks(plan)
	if len(checks) != 8 {
		t.Fatalf("expected 8 checks, got %d: %+v", len(checks), checks)
	}
	ids := map[string]bool{}
	for _, check := range checks {
		if check.ID == "" || ids[check.ID] {
			t.Fatalf("empty or duplicate check ID: %+v", checks)
		}
		ids[check.ID] = true
	}
	if checks[0].Kind != KindDHCP || checks[1].Kind != KindDNS {
		t.Fatalf("unexpected order: %+v", checks)
	}
	if checks[6].Kind != KindHTTP || checks[7].Kind != KindSMB {
		t.Fatalf("unexpected tail order: %+v", checks)
	}
}

func TestDiagnoseMapsResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusInternalServerError)
	}))
	defer badServer.Close()
	adapter := inventory.AdapterConfig{Index: 1, Name: "Ethernet", Operational: true, IPv4Enabled: true, DHCPv4Enabled: true, Gateways: []string{"10.0.0.1"}, DNSServers: []string{"10.0.0.1"}}
	deps := Dependencies{
		Resolver: &fakeResolver{hosts: map[string][]string{"example.test": {"10.1.1.1", "10.1.1.2"}}},
		Dialer:   &fakeDialer{refuse: map[string]bool{"10.0.0.2:445": true}},
		Pinger:   &fakePinger{reachable: map[string]bool{"10.0.0.1": true, "10.0.0.9": false}},
		HTTP:     server.Client(),
	}
	checks := []Check{
		{ID: "dhcp", Kind: KindDHCP, Adapter: &adapter, Timeout: time.Second},
		{ID: "dns", Kind: KindDNS, DNSName: "example.test", Timeout: time.Second, Required: true},
		{ID: "gateway", Kind: KindGateway, Host: "10.0.0.1", Timeout: time.Second},
		{ID: "tcp-ok", Kind: KindTCP, Host: "10.0.0.2", Port: 445, Timeout: time.Second},
		{ID: "tcp-fail", Kind: KindTCP, Host: "10.0.0.2", Port: 445, Timeout: time.Second},
		{ID: "icmp-ok", Kind: KindICMP, Host: "10.0.0.1", Timeout: time.Second},
		{ID: "http-ok", Kind: KindHTTP, URL: server.URL, Timeout: time.Second},
		{ID: "http-fail", Kind: KindHTTP, URL: badServer.URL, Timeout: time.Second},
	}
	// The fake dialer refuses one address; use a second fake for a successful port.
	deps.Dialer = &fakeDialer{refuse: map[string]bool{"10.0.0.2:445": true, "10.0.0.3:445": false}}
	checks[3].Host = "10.0.0.3"
	report, err := Diagnose(context.Background(), checks, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Complete || len(report.Results) != len(checks) {
		t.Fatalf("unexpected report: %+v", report)
	}
	statuses := map[string]Status{}
	for _, result := range report.Results {
		statuses[result.ID] = result.Status
	}
	want := map[string]Status{
		"dhcp": StatusOK, "dns": StatusOK, "gateway": StatusOK, "tcp-ok": StatusOK,
		"tcp-fail": StatusFailed, "icmp-ok": StatusOK, "http-ok": StatusOK, "http-fail": StatusFailed,
	}
	for id, status := range want {
		if statuses[id] != status {
			t.Errorf("%s status = %s, want %s", id, statuses[id], status)
		}
	}
	if report.Failed != 2 {
		t.Fatalf("failed count = %d, want 2", report.Failed)
	}
}

func TestDHCPVariants(t *testing.T) {
	cases := []struct {
		name    string
		adapter inventory.AdapterConfig
		want    Status
	}{
		{"static", inventory.AdapterConfig{Operational: true, IPv4Enabled: true, DHCPv4Enabled: false}, StatusOK},
		{"incomplete", inventory.AdapterConfig{Operational: true, IPv4Enabled: true, DHCPv4Enabled: true}, StatusFailed},
		{"down", inventory.AdapterConfig{Operational: false, IPv4Enabled: true, DHCPv4Enabled: true}, StatusSkipped},
		{"ipv6-only", inventory.AdapterConfig{Operational: true, IPv4Enabled: false}, StatusSkipped},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			adapter := test.adapter
			status, code, _ := checkDHCP(context.Background(), Check{Adapter: &adapter})
			if status != test.want {
				t.Fatalf("status = %s (%s), want %s", status, code, test.want)
			}
		})
	}
}

func TestDiagnoseReportsTimeout(t *testing.T) {
	deps := Dependencies{Resolver: &fakeResolver{delay: 100 * time.Millisecond}}
	checks := []Check{{ID: "slow-dns", Kind: KindDNS, DNSName: "example.test", Timeout: 20 * time.Millisecond}}
	report, err := Diagnose(context.Background(), checks, deps)
	if err != nil {
		t.Fatal(err)
	}
	if report.Results[0].Status != StatusTimeout || report.Results[0].ErrorCode != "timeout" {
		t.Fatalf("unexpected timeout result: %+v", report.Results[0])
	}
}

func TestDiagnoseHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := Diagnose(ctx, []Check{{ID: "dns", Kind: KindDNS, DNSName: "example.test"}}, Dependencies{})
	if !errors.Is(err, context.Canceled) || len(report.Results) != 0 {
		t.Fatalf("unexpected cancellation: %+v, %v", report, err)
	}
}

func TestDiagnoseUnsupportedDependencies(t *testing.T) {
	checks := []Check{
		{ID: "dns", Kind: KindDNS, DNSName: "example.test"},
		{ID: "tcp", Kind: KindTCP, Host: "host", Port: 445},
		{ID: "icmp", Kind: KindICMP, Host: "host"},
		{ID: "http", Kind: KindHTTP, URL: "https://example.test"},
	}
	report, err := Diagnose(context.Background(), checks, Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range report.Results {
		if result.Status != StatusUnsupported && result.Status != StatusFailed {
			t.Fatalf("unexpected status without dependencies: %+v", result)
		}
	}
}

func TestSanitizeID(t *testing.T) {
	if got := sanitizeID("\\\\server\\share_name 1"); strings.ContainsAny(got, "\\ /") || got == "" {
		t.Fatalf("unsafe ID: %q", got)
	}
}
