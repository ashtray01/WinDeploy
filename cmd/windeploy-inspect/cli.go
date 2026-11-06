package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"windeploy/core/drivers"
	"windeploy/core/inventory"
	"windeploy/core/network"
	"windeploy/core/printers"
	"windeploy/core/profiles"
)

var diagnosticDependencies = network.NativeDependencies()

func usage(writer io.Writer) {
	fmt.Fprintln(writer, "WinDeploy read-only inspection\n  inventory [--format text|json] [--output NEW-FILE] [--timeout 15s]\n  network [--dns name] [--tcp host:port] [--icmp host] [--http url] [--smb host] [--json] [--timeout 5s] [--output NEW-FILE]\n  devices [--problems] [--json] [--output NEW-FILE]\n  printers [--json] [--output NEW-FILE]\n  preview --profile ID [--profiles profiles] [--select id,id] [--output NEW-FILE]\nExit codes: 0 complete, 1 input/output error, 2 partial inventory/failed diagnostics, 3 cancelled/deadline.\nInventory, devices and printers include private workstation metadata; keep reports local.")
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, provider inventory.Provider) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		usage(stdout)
		return 0
	}
	switch args[0] {
	case "inventory":
		return runInventory(ctx, args[1:], stdout, stderr, provider)
	case "preview":
		return runPreview(ctx, args[1:], stdout, stderr)
	case "network":
		return runNetwork(ctx, args[1:], stdout, stderr)
	case "devices":
		return runDevices(ctx, args[1:], stdout, stderr)
	case "printers":
		return runPrinters(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "unknown command; use help")
		return 1
	}
}

func runInventory(ctx context.Context, args []string, stdout, stderr io.Writer, provider inventory.Provider) int {
	flags := flag.NewFlagSet("inventory", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "text or json")
	output := flags.String("output", "", "create a new local report file; never overwrite")
	timeout := flags.Duration("timeout", 15*time.Second, "cooperative collection timeout")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 || (*format != "text" && *format != "json") || *timeout <= 0 {
		fmt.Fprintln(stderr, "invalid inventory options")
		return 1
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	if provider == nil {
		provider = inventory.Native{}
	}
	report, collectErr := inventory.Collect(ctx, provider)
	var data []byte
	var err error
	if *format == "json" {
		data, err = encodeJSON(report)
	} else {
		var text bytes.Buffer
		fmt.Fprintf(&text, "WinDeploy inventory (read-only)\nComplete: %t\n", report.Complete)
		if report.System != nil {
			fmt.Fprintf(&text, "Host: %s\nOS: %s\nNative architecture: %s (process: %s)\n", report.System.Hostname, report.System.Windows.Label(), report.System.Architecture, report.System.ProcessArchitecture)
		}
		if report.CPU != nil {
			fmt.Fprintf(&text, "CPU: %s; logical processors: %d\n", report.CPU.Model, report.CPU.LogicalProcessors)
		}
		if report.Memory != nil {
			fmt.Fprintf(&text, "RAM bytes: total=%d available=%d\n", report.Memory.TotalPhysicalBytes, report.Memory.AvailablePhysicalBytes)
		}
		fmt.Fprintf(&text, "Volumes: %d; interfaces: %d; adapters: %d; registered apps: %d\n", len(report.Volumes), len(report.Interfaces), len(report.Network), len(report.Apps))
		for _, section := range report.Sections {
			fmt.Fprintf(&text, "%s: %s\n", section.Name, section.Status)
		}
		data = text.Bytes()
	}
	if err != nil || writeOutput(*output, data, stdout) != nil {
		fmt.Fprintln(stderr, "could not write inventory report")
		return 1
	}
	if errors.Is(collectErr, context.Canceled) || errors.Is(collectErr, context.DeadlineExceeded) {
		fmt.Fprintln(stderr, "inventory cancelled; report contains partial results")
		return 3
	}
	if errors.Is(collectErr, inventory.ErrIncomplete) {
		fmt.Fprintln(stderr, "inventory incomplete; inspect section statuses")
		return 2
	}
	if collectErr != nil {
		fmt.Fprintln(stderr, "inventory failed")
		return 1
	}
	return 0
}

func runPreview(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("preview", flag.ContinueOnError)
	flags.SetOutput(stderr)
	directory := flags.String("profiles", "profiles", "explicit profile directory")
	id := flags.String("profile", "", "profile ID")
	selected := flags.String("select", "", "component IDs; explicitly empty selects none")
	output := flags.String("output", "", "create a new local report file; never overwrite")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *id == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "profile ID is required; use preview --help")
		return 1
	}
	registry, err := loadProfiles(ctx, *directory)
	if err != nil {
		fmt.Fprintln(stderr, "profile directory could not be loaded")
		if ctx.Err() != nil {
			return 3
		}
		return 1
	}
	profile, err := profiles.Resolve(registry, *id)
	if err != nil {
		fmt.Fprintln(stderr, "profile inheritance is invalid")
		return 1
	}
	var selection []string
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "select" {
			selection = []string{}
			if strings.TrimSpace(*selected) != "" {
				for _, value := range strings.Split(*selected, ",") {
					selection = append(selection, strings.TrimSpace(value))
				}
			}
		}
	})
	preview, err := profiles.Preview(profile, selection)
	if err != nil {
		fmt.Fprintln(stderr, "profile selection or dependency plan is invalid")
		return 1
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "preview cancelled")
		return 3
	}
	data, err := encodeJSON(preview)
	if err != nil || writeOutput(*output, data, stdout) != nil {
		fmt.Fprintln(stderr, "could not write preview")
		return 1
	}
	return 0
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("empty value")
	}
	*s = append(*s, strings.TrimSpace(value))
	return nil
}

func runNetwork(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("network", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var dnsNames, tcpTargets, icmpHosts, httpURLs, smbHosts stringList
	flags.Var(&dnsNames, "dns", "DNS name to resolve (repeatable)")
	flags.Var(&tcpTargets, "tcp", "TCP host:port (repeatable)")
	flags.Var(&icmpHosts, "icmp", "ICMP host (repeatable)")
	flags.Var(&httpURLs, "http", "HTTP(S) URL (repeatable)")
	flags.Var(&smbHosts, "smb", "SMB server for port 445 (repeatable)")
	timeout := flags.Duration("timeout", 5*time.Second, "per-check timeout")
	asJSON := flags.Bool("json", false, "JSON output")
	output := flags.String("output", "", "create a new local report file; never overwrite")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 || *timeout <= 0 {
		fmt.Fprintln(stderr, "invalid network options")
		return 1
	}
	configCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	adapters, adapterErr := inventory.Native{}.NetworkConfig(configCtx)
	cancel()
	if adapterErr != nil {
		fmt.Fprintln(stderr, "network adapters could not be read")
		return 1
	}
	endpoints := []network.Endpoint{}
	for _, target := range tcpTargets {
		host, portText, err := net.SplitHostPort(target)
		if err != nil {
			fmt.Fprintf(stderr, "invalid TCP target %q; use host:port\n", target)
			return 1
		}
		port, convErr := strconv.Atoi(portText)
		if convErr != nil || port <= 0 || port > 65535 {
			fmt.Fprintf(stderr, "invalid TCP port in %q\n", target)
			return 1
		}
		endpoints = append(endpoints, network.Endpoint{Host: host, Port: port})
	}
	gateways := []string{}
	servers := []string{}
	for _, adapter := range adapters {
		if !adapter.Operational {
			continue
		}
		gateways = append(gateways, adapter.Gateways...)
		servers = append(servers, adapter.DNSServers...)
	}
	plan := network.Plan{
		Adapters: adapters, DNSNames: dnsNames, Servers: servers, Gateways: gateways,
		TCPTargets: endpoints, ICMPHosts: icmpHosts, HTTPURLs: httpURLs, SMBHosts: smbHosts, Timeout: *timeout,
	}
	checks := network.BuildChecks(plan)
	if len(checks) == 0 {
		fmt.Fprintln(stderr, "no checks selected; add --dns, --tcp, --icmp, --http or --smb")
		return 1
	}
	runCtx, stop := context.WithTimeout(ctx, time.Duration(len(checks))*(*timeout+time.Second)+5*time.Second)
	defer stop()
	report, err := network.Diagnose(runCtx, checks, diagnosticDependencies)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		fmt.Fprintln(stderr, "diagnostics failed")
		return 1
	}
	var data []byte
	if *asJSON {
		data, err = encodeJSON(report)
	} else {
		var text bytes.Buffer
		fmt.Fprintf(&text, "WinDeploy network diagnostics\nChecks: %d; failed: %d; complete: %t\n", len(report.Results), report.Failed, report.Complete)
		for _, result := range report.Results {
			detail := result.Detail
			if detail == "" {
				detail = result.ErrorCode
			}
			fmt.Fprintf(&text, "[%s] %s — %s (%d ms)\n", result.Status, result.Name, detail, result.LatencyMS)
		}
		data = text.Bytes()
	}
	if err != nil || writeOutput(*output, data, stdout) != nil {
		fmt.Fprintln(stderr, "could not write diagnostics")
		return 1
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return 3
	}
	if report.Failed > 0 || !report.Complete {
		return 2
	}
	return 0
}

func runDevices(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("devices", flag.ContinueOnError)
	flags.SetOutput(stderr)
	problemsOnly := flags.Bool("problems", false, "only devices with a problem code")
	asJSON := flags.Bool("json", false, "JSON output")
	output := flags.String("output", "", "create a new local report file; never overwrite")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "invalid devices options")
		return 1
	}
	runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	devices, err := (drivers.WindowsDevices{}).Devices(runCtx)
	if err != nil {
		fmt.Fprintln(stderr, "devices could not be enumerated")
		return 1
	}
	filtered := make([]drivers.Device, 0, len(devices))
	for _, device := range devices {
		if *problemsOnly && device.Status != "problem" {
			continue
		}
		filtered = append(filtered, device)
	}
	var data []byte
	if *asJSON {
		data, err = encodeJSON(filtered)
	} else {
		var text bytes.Buffer
		fmt.Fprintf(&text, "WinDeploy devices (read-only)\nTotal: %d; shown: %d\n", len(devices), len(filtered))
		for _, device := range filtered {
			fmt.Fprintf(&text, "[%s] %s | %s | driver=%s %s | problem=%d\n", device.Status, device.Name, device.Class, device.DriverVersion, device.DriverProvider, device.ProblemCode)
			for _, hwid := range device.HardwareIDs {
				fmt.Fprintf(&text, "    %s\n", hwid)
			}
		}
		data = text.Bytes()
	}
	if err != nil || writeOutput(*output, data, stdout) != nil {
		fmt.Fprintln(stderr, "could not write devices report")
		return 1
	}
	return 0
}

func runPrinters(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("printers", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "JSON output")
	output := flags.String("output", "", "create a new local report file; never overwrite")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "invalid printers options")
		return 1
	}
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	items, err := (printers.WindowsPrinters{}).Printers(runCtx)
	if err != nil {
		fmt.Fprintln(stderr, "printers could not be enumerated")
		return 1
	}
	defaultName, _ := (printers.WindowsDefaults{}).Default(runCtx)
	var data []byte
	if *asJSON {
		data, err = encodeJSON(map[string]any{"printers": items, "default": defaultName})
	} else {
		var text bytes.Buffer
		fmt.Fprintf(&text, "WinDeploy printers (read-only)\nTotal: %d; default: %s\n", len(items), defaultName)
		for _, printer := range items {
			fmt.Fprintf(&text, "[%s] %s | driver=%s | port=%s | shared=%t\n", printer.Status, printer.Name, printer.Driver, printer.Port, printer.Shared)
		}
		data = text.Bytes()
	}
	if err != nil || writeOutput(*output, data, stdout) != nil {
		fmt.Fprintln(stderr, "could not write printers report")
		return 1
	}
	return 0
}

func loadProfiles(ctx context.Context, directory string) (map[string]profiles.Profile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	registry := map[string]profiles.Profile{}
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, errors.New("profile entries must be regular files")
		}
		if len(registry) >= 128 {
			return nil, errors.New("too many profiles")
		}
		file, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		profile, err := profiles.Load(file)
		closeErr := file.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if _, exists := registry[profile.ID]; exists {
			return nil, errors.New("duplicate profile ID")
		}
		registry[profile.ID] = profile
	}
	return registry, nil
}

func encodeJSON(value interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Exclusive creation prevents accidental replacement (including symlink targets).
// Failed writes remove only the file this invocation just created.
func writeOutput(path string, data []byte, stdout io.Writer) error {
	if path == "" {
		n, err := stdout.Write(data)
		if err == nil && n != len(data) {
			err = io.ErrShortWrite
		}
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, err := file.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}
