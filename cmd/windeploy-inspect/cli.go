package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"windeploy/core/inventory"
	"windeploy/core/profiles"
)

func usage(writer io.Writer) {
	fmt.Fprintln(writer, "WinDeploy read-only inspection\n  inventory [--format text|json] [--output NEW-FILE] [--timeout 15s]\n  preview --profile ID [--profiles profiles] [--select id,id] [--output NEW-FILE]\nExit codes: 0 complete, 1 input/output error, 2 partial inventory, 3 cancelled/deadline.\nInventory includes private workstation metadata; keep reports local.")
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
