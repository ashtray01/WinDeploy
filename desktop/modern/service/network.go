package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"windeploy/core/network"
)

// NetworkCheckView is one translated diagnostic outcome.
type NetworkCheckView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
	Required  bool   `json:"required"`
}

// NetworkReportView is the package page state for network diagnostics.
type NetworkReportView struct {
	Checks   []NetworkCheckView `json:"checks"`
	Failed   int                `json:"failed"`
	Complete bool               `json:"complete"`
	Message  string             `json:"message"`
}

type networkTarget struct {
	kind  network.Kind
	host  string
	port  int
	url   string
	label string
}

// RunNetworkDiagnostics performs read-only checks. Targets use the form
// "dns:name", "tcp:host:port", "icmp:host", "http:url" or "smb:host".
func (s *Service) RunNetworkDiagnostics(targets []string) (NetworkReportView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	configCtx, configCancel := context.WithTimeout(ctx, 10*time.Second)
	adapters, err := s.provider.NetworkConfig(configCtx)
	configCancel()
	if err != nil {
		return NetworkReportView{}, errors.New("Не удалось прочитать настройки сетевых адаптеров.")
	}
	plan := network.Plan{Adapters: adapters, Timeout: 4 * time.Second}
	for _, raw := range targets {
		target, parseErr := parseNetworkTarget(raw)
		if parseErr != nil {
			return NetworkReportView{}, parseErr
		}
		switch target.kind {
		case network.KindDNS:
			plan.DNSNames = append(plan.DNSNames, target.host)
			for _, adapter := range adapters {
				if adapter.Operational {
					plan.Servers = append(plan.Servers, adapter.DNSServers...)
				}
			}
		case network.KindTCP:
			plan.TCPTargets = append(plan.TCPTargets, network.Endpoint{Host: target.host, Port: target.port})
		case network.KindICMP:
			plan.ICMPHosts = append(plan.ICMPHosts, target.host)
		case network.KindHTTP:
			plan.HTTPURLs = append(plan.HTTPURLs, target.url)
		case network.KindSMB:
			plan.SMBHosts = append(plan.SMBHosts, target.host)
		}
	}
	for _, adapter := range adapters {
		if adapter.Operational {
			plan.Gateways = append(plan.Gateways, adapter.Gateways...)
		}
	}
	checks := network.BuildChecks(plan)
	if len(checks) == 0 {
		return NetworkReportView{}, errors.New("Не выбрано ни одной проверки.")
	}
	report, diagnoseErr := network.Diagnose(ctx, checks, network.NativeDependencies())
	view := NetworkReportView{Checks: make([]NetworkCheckView, 0, len(report.Results)), Failed: report.Failed, Complete: report.Complete}
	for _, result := range report.Results {
		view.Checks = append(view.Checks, NetworkCheckView{ID: result.ID, Name: result.Name, Kind: string(result.Kind), Status: string(result.Status), Detail: result.Detail, ErrorCode: result.ErrorCode, LatencyMS: result.LatencyMS, Required: result.Required})
	}
	switch {
	case errors.Is(diagnoseErr, context.DeadlineExceeded), errors.Is(diagnoseErr, context.Canceled):
		view.Message = "Диагностика прервана по времени. Показаны выполненные проверки."
	case view.Failed > 0:
		view.Message = fmt.Sprintf("Проверки завершены. Ошибок: %d из %d.", view.Failed, len(view.Checks))
	default:
		view.Message = "Все проверки успешно выполнены."
	}
	return view, nil
}

func parseNetworkTarget(raw string) (networkTarget, error) {
	raw = strings.TrimSpace(raw)
	index := strings.Index(raw, ":")
	if index <= 0 {
		return networkTarget{}, errors.New("Некорректная цель диагностики.")
	}
	kind := strings.ToLower(raw[:index])
	value := strings.TrimSpace(raw[index+1:])
	switch kind {
	case "dns":
		if value == "" {
			return networkTarget{}, errors.New("Укажите имя для DNS-проверки.")
		}
		return networkTarget{kind: network.KindDNS, host: value}, nil
	case "icmp":
		if value == "" {
			return networkTarget{}, errors.New("Укажите узел для ICMP-проверки.")
		}
		return networkTarget{kind: network.KindICMP, host: value}, nil
	case "smb":
		if value == "" {
			return networkTarget{}, errors.New("Укажите сервер для SMB-проверки.")
		}
		return networkTarget{kind: network.KindSMB, host: value}, nil
	case "http":
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return networkTarget{}, errors.New("HTTP-проверка требует адрес http:// или https://.")
		}
		return networkTarget{kind: network.KindHTTP, url: value}, nil
	case "tcp":
		host, port, ok := splitHostPort(value)
		if !ok {
			return networkTarget{}, errors.New("TCP-проверка требует host:port.")
		}
		return networkTarget{kind: network.KindTCP, host: host, port: port}, nil
	default:
		return networkTarget{}, errors.New("Поддерживаются цели dns, tcp, icmp, http и smb.")
	}
}

func splitHostPort(value string) (string, int, bool) {
	host, portText, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" || portText == "" {
		return "", 0, false
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, false
	}
	return host, port, true
}
