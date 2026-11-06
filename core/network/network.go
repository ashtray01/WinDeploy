// Package network performs bounded, read-only connectivity diagnostics. It never
// changes adapter configuration and never stores credentials.
package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"windeploy/core/inventory"
)

// Kind is a diagnostic category.
type Kind string

const (
	KindDHCP      Kind = "dhcp"
	KindDNS       Kind = "dns"
	KindDNSServer Kind = "dns-server"
	KindGateway   Kind = "gateway"
	KindTCP       Kind = "tcp"
	KindICMP      Kind = "icmp"
	KindHTTP      Kind = "http"
	KindSMB       Kind = "smb"
)

// Status is the outcome of one check.
type Status string

const (
	StatusOK          Status = "ok"
	StatusFailed      Status = "failed"
	StatusTimeout     Status = "timeout"
	StatusSkipped     Status = "skipped"
	StatusUnsupported Status = "unsupported"
)

// Check is one bounded read-only diagnostic.
type Check struct {
	ID      string
	Name    string
	Kind    Kind
	Host    string
	Port    int
	URL     string
	DNSName string
	Server  string
	Adapter *inventory.AdapterConfig
	Timeout time.Duration
	// Required marks a failed check that makes the whole report incomplete.
	Required bool
}

// Result is a safe diagnostic outcome. Detail contains no credentials.
type Result struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kind       Kind      `json:"kind"`
	Status     Status    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	LatencyMS  int64     `json:"latency_ms"`
	Detail     string    `json:"detail,omitempty"`
	ErrorCode  string    `json:"error_code,omitempty"`
	Required   bool      `json:"required"`
}

// Report is the complete diagnostic outcome. Failed individual checks are data,
// not a transport error; only invalid input or cancellation returns an error.
type Report struct {
	Version    int       `json:"version"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Results    []Result  `json:"results"`
	Complete   bool      `json:"complete"`
	Failed     int       `json:"failed"`
}

// Resolver resolves names with the system resolver.
type Resolver interface {
	LookupHost(context.Context, string) ([]string, error)
}

// ServerResolver queries a specific DNS server.
type ServerResolver interface {
	LookupWithServer(context.Context, string, string) ([]string, error)
}

// Dialer opens TCP connections.
type Dialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// Pinger performs one ICMP echo.
type Pinger interface {
	Ping(context.Context, string) (time.Duration, error)
}

// HTTPDoer performs an HTTP request.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Dependencies are the injectable diagnostics primitives.
type Dependencies struct {
	Resolver       Resolver
	ServerResolver ServerResolver
	Dialer         Dialer
	Pinger         Pinger
	HTTP           HTTPDoer
}

// ErrCheckTimeout distinguishes a per-check timeout from a failed result.
var ErrCheckTimeout = errors.New("diagnostic check timed out")

// ErrPingUnavailable reports that the platform ping helper could not be started.
var ErrPingUnavailable = errors.New("icmp helper is unavailable")

// Diagnose runs checks in order with a per-check deadline. Cancellation of the
// parent context stops the run and returns the partial report with ctx.Err().
func Diagnose(ctx context.Context, checks []Check, deps Dependencies) (Report, error) {
	if len(checks) > 256 {
		return Report{}, fmt.Errorf("too many network checks")
	}
	report := Report{Version: 1, StartedAt: time.Now().UTC(), Results: make([]Result, 0, len(checks))}
	for _, check := range checks {
		if err := ctx.Err(); err != nil {
			report.FinishedAt = time.Now().UTC()
			return report, err
		}
		result := runCheck(ctx, check, deps)
		report.Results = append(report.Results, result)
		if result.Status == StatusFailed || result.Status == StatusTimeout || result.Status == StatusUnsupported {
			report.Failed++
		}
	}
	report.Complete = true
	report.FinishedAt = time.Now().UTC()
	return report, nil
}

func runCheck(parent context.Context, check Check, deps Dependencies) Result {
	result := Result{ID: check.ID, Name: check.Name, Kind: check.Kind, Required: check.Required, Status: StatusFailed, StartedAt: time.Now().UTC()}
	timeout := check.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	start := time.Now()
	status, code, detail := execute(ctx, check, deps)
	elapsed := time.Since(start)
	result.Status, result.ErrorCode, result.Detail = status, code, detail
	result.LatencyMS = elapsed.Milliseconds()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) && parent.Err() == nil {
		result.Status, result.ErrorCode = StatusTimeout, "timeout"
		if result.Detail == "" {
			result.Detail = "Превышено время ожидания"
		}
	}
	result.FinishedAt = time.Now().UTC()
	return result
}

func execute(ctx context.Context, check Check, deps Dependencies) (Status, string, string) {
	switch check.Kind {
	case KindDHCP:
		return checkDHCP(ctx, check)
	case KindDNS:
		if deps.Resolver == nil {
			return StatusUnsupported, "resolver-unavailable", "Системный resolver недоступен"
		}
		addresses, err := deps.Resolver.LookupHost(ctx, check.DNSName)
		if err != nil {
			return StatusFailed, "dns-failed", "Не удалось разрешить имя"
		}
		return StatusOK, "", fmt.Sprintf("Адресов: %d", len(addresses))
	case KindDNSServer:
		if deps.ServerResolver == nil {
			return StatusUnsupported, "resolver-unavailable", "Проверка DNS-сервера недоступна"
		}
		addresses, err := deps.ServerResolver.LookupWithServer(ctx, check.Server, check.DNSName)
		if err != nil {
			return StatusFailed, "dns-server-failed", "DNS-сервер не ответил"
		}
		return StatusOK, "", fmt.Sprintf("Ответил адресов: %d", len(addresses))
	case KindGateway, KindICMP:
		if deps.Pinger == nil {
			return StatusUnsupported, "ping-unavailable", "ICMP недоступен"
		}
		latency, err := deps.Pinger.Ping(ctx, check.Host)
		if err != nil {
			if errors.Is(err, ErrPingUnavailable) {
				return StatusUnsupported, "ping-unavailable", "ICMP-помощник недоступен"
			}
			return StatusFailed, "icmp-failed", "Узел не ответил на ICMP"
		}
		return StatusOK, "", fmt.Sprintf("Ответ за %d мс", latency.Milliseconds())
	case KindTCP, KindSMB:
		if deps.Dialer == nil {
			return StatusUnsupported, "dialer-unavailable", "TCP-подключение недоступно"
		}
		address := net.JoinHostPort(check.Host, fmt.Sprintf("%d", check.Port))
		connection, err := deps.Dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			code := "tcp-failed"
			if check.Kind == KindSMB {
				code = "smb-failed"
			}
			return StatusFailed, code, "Порт недоступен"
		}
		_ = connection.Close()
		if check.Kind == KindSMB {
			return StatusOK, "", fmt.Sprintf("SMB-порт %d доступен", check.Port)
		}
		return StatusOK, "", fmt.Sprintf("TCP-порт %d доступен", check.Port)
	case KindHTTP:
		if deps.HTTP == nil {
			return StatusUnsupported, "http-unavailable", "HTTP-клиент недоступен"
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, check.URL, nil)
		if err != nil {
			return StatusFailed, "http-invalid", "Некорректный URL"
		}
		request.Header.Set("User-Agent", "WinDeploy-Diagnostics/1.0")
		response, err := deps.HTTP.Do(request)
		if err != nil {
			return StatusFailed, "http-failed", "HTTP-сервер недоступен"
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 400 {
			return StatusFailed, "http-status", fmt.Sprintf("HTTP-статус %d", response.StatusCode)
		}
		return StatusOK, "", fmt.Sprintf("HTTP-статус %d", response.StatusCode)
	default:
		return StatusSkipped, "unknown-kind", "Неизвестный тип проверки"
	}
}

func checkDHCP(ctx context.Context, check Check) (Status, string, string) {
	if err := ctx.Err(); err != nil {
		return StatusFailed, "cancelled", "Проверка отменена"
	}
	adapter := check.Adapter
	if adapter == nil {
		return StatusSkipped, "adapter-missing", "Адаптер не передан"
	}
	if !adapter.Operational {
		return StatusSkipped, "adapter-down", "Адаптер не подключён"
	}
	if !adapter.IPv4Enabled {
		return StatusSkipped, "ipv4-disabled", "IPv4 отключён"
	}
	if !adapter.DHCPv4Enabled {
		return StatusOK, "", "IPv4 настроен статически"
	}
	var missing []string
	if len(adapter.Gateways) == 0 {
		missing = append(missing, "шлюз")
	}
	if len(adapter.DNSServers) == 0 {
		missing = append(missing, "DNS")
	}
	if len(missing) > 0 {
		return StatusFailed, "dhcp-incomplete", "DHCP не выдал: " + strings.Join(missing, ", ")
	}
	return StatusOK, "", "DHCP выдал шлюз и DNS"
}

// Plan describes the checks derived from the current inventory and explicit
// targets. Empty fields produce no checks.
type Plan struct {
	Adapters   []inventory.AdapterConfig
	DNSNames   []string
	Servers    []string
	Gateways   []string
	TCPTargets []Endpoint
	ICMPHosts  []string
	HTTPURLs   []string
	SMBHosts   []string
	Timeout    time.Duration
}

// Endpoint is a host/port target.
type Endpoint struct {
	Host string
	Port int
}

// BuildChecks converts a plan into ordered checks with stable IDs.
func BuildChecks(plan Plan) []Check {
	timeout := plan.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	checks := []Check{}
	adapters := append([]inventory.AdapterConfig{}, plan.Adapters...)
	sort.SliceStable(adapters, func(i, j int) bool { return adapters[i].Index < adapters[j].Index })
	for i := range adapters {
		adapter := adapters[i]
		checks = append(checks, Check{ID: fmt.Sprintf("dhcp-%d", adapter.Index), Name: "DHCP · " + adapter.Name, Kind: KindDHCP, Adapter: &adapter, Timeout: timeout, Required: false})
	}
	for _, name := range dedupe(plan.DNSNames) {
		checks = append(checks, Check{ID: "dns-" + sanitizeID(name), Name: "DNS · " + name, Kind: KindDNS, DNSName: name, Timeout: timeout, Required: true})
	}
	for _, server := range dedupe(plan.Servers) {
		for index, name := range dedupe(plan.DNSNames) {
			if index > 0 {
				break
			}
			checks = append(checks, Check{ID: "dns-server-" + sanitizeID(server), Name: "DNS-сервер · " + server, Kind: KindDNSServer, Server: server, DNSName: name, Timeout: timeout, Required: false})
		}
	}
	for _, gateway := range dedupe(plan.Gateways) {
		checks = append(checks, Check{ID: "gateway-" + sanitizeID(gateway), Name: "Шлюз · " + gateway, Kind: KindGateway, Host: gateway, Timeout: timeout, Required: true})
	}
	for _, endpoint := range plan.TCPTargets {
		if endpoint.Host == "" || endpoint.Port <= 0 || endpoint.Port > 65535 {
			continue
		}
		checks = append(checks, Check{ID: "tcp-" + sanitizeID(endpoint.Host) + "-" + fmt.Sprint(endpoint.Port), Name: fmt.Sprintf("TCP · %s:%d", endpoint.Host, endpoint.Port), Kind: KindTCP, Host: endpoint.Host, Port: endpoint.Port, Timeout: timeout, Required: false})
	}
	for _, host := range dedupe(plan.ICMPHosts) {
		checks = append(checks, Check{ID: "icmp-" + sanitizeID(host), Name: "ICMP · " + host, Kind: KindICMP, Host: host, Timeout: timeout, Required: false})
	}
	for _, rawURL := range dedupe(plan.HTTPURLs) {
		checks = append(checks, Check{ID: "http-" + sanitizeID(rawURL), Name: "HTTP · " + rawURL, Kind: KindHTTP, URL: rawURL, Timeout: timeout, Required: false})
	}
	for _, host := range dedupe(plan.SMBHosts) {
		checks = append(checks, Check{ID: "smb-" + sanitizeID(host), Name: "SMB · " + host, Kind: KindSMB, Host: host, Port: 445, Timeout: timeout, Required: false})
	}
	return checks
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func sanitizeID(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
		if builder.Len() >= 48 {
			break
		}
	}
	return strings.Trim(builder.String(), "-")
}
