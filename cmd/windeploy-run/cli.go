package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	profilecatalog "windeploy/catalog"
	"windeploy/core/browsers"
	"windeploy/core/deploy"
	"windeploy/core/drivers"
	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/network"
	"windeploy/core/packages"
	"windeploy/core/printers"
	coreprofiles "windeploy/core/profiles"
	"windeploy/core/reporting"
	"windeploy/core/settings"
	"windeploy/core/shares"
	profileassets "windeploy/profiles"
)

const (
	exitOK      = 0
	exitUsage   = 2
	exitPlan    = 3
	exitSession = 4
	exitFailed  = 5
	exitConfirm = 7
)

type environment struct {
	Adapter    engine.Adapter
	ReadSystem func(context.Context) (inventory.SystemInfo, error)
	Sessions   string
	Reports    string
	System     inventory.SystemInfo
	RunOptions engine.RunOptions
}

func buildEnvironment(base string, system inventory.SystemInfo) (environment, error) {
	catalog, err := loadCatalog()
	if err != nil {
		return environment{}, err
	}
	facts := packages.WindowsFacts{}
	packagesManager := &packages.Manager{
		Catalog: catalog, Facts: facts, System: system, Winget: &packages.WindowsWinget{},
		Fetcher: packages.NewHTTPFetcher(), Runner: packages.WindowsRunner{}, Signer: packages.WindowsSigner{},
		Elevation: packages.WindowsElevation{}, CacheDir: filepath.Join(base, "Downloads"),
	}
	desktop, _ := shares.DesktopDirectory()
	shareManager := &shares.Manager{Store: shares.WindowsCredentialStore{}, Connector: shares.WindowsConnector{}, Shortcuts: shares.WindowsShortcut{}}
	driverManager := &drivers.Manager{Devices: drivers.WindowsDevices{}, Installer: drivers.WindowsInstaller{Runner: drivers.WindowsRunner{}, System: system}, Verifier: drivers.WindowsINFVerifier{}, Elevation: drivers.WindowsElevation{}}
	printerManager := &printers.Manager{Printers: printers.WindowsPrinters{}, Installer: printers.WindowsInstaller{}, Defaults: printers.WindowsDefaults{}, Tester: printers.WindowsTestPrinter{}, Elevation: printers.WindowsElevation{}}
	browserManager := &browsers.Manager{Store: browsers.NewRegistryStore(), Shortcuts: browsers.WindowsShortcut{}, Locator: browsers.WindowsLocator{}, Elevation: browsers.WindowsElevation{}, DesktopDir: desktop}
	settingsManager := &settings.Manager{Registry: settings.WindowsRegistry{}, Runner: settings.WindowsRunner{}, Services: settings.WindowsServices{}, Elevation: settings.WindowsElevation{}, Activation: settings.WindowsActivation{}, Computer: settings.WindowsComputerName{}, System: system}
	adapter := &deploy.Adapter{
		PackageManager: packagesManager, Shares: shareManager, Drivers: driverManager, Printers: printerManager,
		Browsers: browserManager, Settings: settingsManager, NetworkDeps: network.NativeDependencies(),
		NetworkConfig: func(ctx context.Context) ([]inventory.AdapterConfig, error) {
			return inventory.Native{}.NetworkConfig(ctx)
		},
		ShortcutDir: desktop, System: system,
	}
	return environment{Adapter: adapter, Sessions: filepath.Join(base, "sessions"), Reports: filepath.Join(base, "reports"), System: system, RunOptions: engine.RunOptions{MaxAttempts: 2, RetryDelay: 2 * time.Second, MaxParallel: 1}}, nil
}

func usage(w io.Writer) {
	fmt.Fprint(w, `WinDeploy deployment runner

  windeploy-run plan    --profile ID [--profiles DIR] [--select a,b] [--json]
  windeploy-run run     --profile ID [--profiles DIR] [--select a,b] [--yes] [--sessions DIR] [--reports DIR]
  windeploy-run resume  --session ID [--retry-failed] [--yes] [--sessions DIR]
  windeploy-run sessions [--sessions DIR] [--json]
  windeploy-run report  --session ID [--sessions DIR] [--reports DIR]

run/resume изменяют систему и требуют явного --yes. Отчёты пишутся в reports DIR.
Профили по умолчанию берутся из встроенного каталога EXE.
`)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, env environment) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		usage(stdout)
		return exitOK
	}
	switch args[0] {
	case "plan":
		return runPlan(ctx, args[1:], stdout, stderr, env)
	case "run":
		return runDeployment(ctx, args[1:], stdout, stderr, env)
	case "resume":
		return runResume(ctx, args[1:], stdout, stderr, env)
	case "sessions":
		return runSessions(ctx, args[1:], stdout, stderr, env)
	case "report":
		return runReport(ctx, args[1:], stdout, stderr, env)
	default:
		fmt.Fprintln(stderr, "неизвестная команда; используйте help")
		return exitUsage
	}
}

func loadCatalogRegistry(directory string) (map[string]coreprofiles.Profile, error) {
	registry := map[string]coreprofiles.Profile{}
	if directory == "" {
		entries, err := fs.ReadDir(profileassets.Files, ".")
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}
			file, err := profileassets.Files.Open(entry.Name())
			if err != nil {
				return nil, err
			}
			profile, loadErr := coreprofiles.Load(file)
			file.Close()
			if loadErr != nil {
				return nil, loadErr
			}
			registry[profile.ID] = profile
		}
		return registry, nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		if len(registry) >= 128 {
			return nil, fmt.Errorf("too many profiles")
		}
		file, err := os.Open(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, err
		}
		profile, loadErr := coreprofiles.Load(file)
		file.Close()
		if loadErr != nil {
			return nil, loadErr
		}
		if _, exists := registry[profile.ID]; exists {
			return nil, fmt.Errorf("duplicate profile ID")
		}
		registry[profile.ID] = profile
	}
	return registry, nil
}

func selectionFromFlag(flags *flag.FlagSet, value string) []string {
	var selection []string
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "select" {
			selection = []string{}
			for _, part := range strings.Split(value, ",") {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					selection = append(selection, trimmed)
				}
			}
		}
	})
	return selection
}

func buildPlan(profileID, directory, selectValue string, flags *flag.FlagSet) (coreprofiles.Profile, []engine.Task, engine.PlanPreview, error) {
	registry, err := loadCatalogRegistry(directory)
	if err != nil {
		return coreprofiles.Profile{}, nil, engine.PlanPreview{}, err
	}
	profile, err := coreprofiles.Resolve(registry, profileID)
	if err != nil {
		return coreprofiles.Profile{}, nil, engine.PlanPreview{}, err
	}
	tasks, err := coreprofiles.BuildPlan(profile, selectionFromFlag(flags, selectValue))
	if err != nil {
		return profile, nil, engine.PlanPreview{}, err
	}
	preview, err := engine.PreviewPlan(tasks)
	if err != nil {
		return profile, nil, engine.PlanPreview{}, err
	}
	return profile, tasks, preview, nil
}

func printPreview(w io.Writer, preview engine.PlanPreview) {
	fmt.Fprintf(w, "План: %d задач (%s)\n", len(preview.Tasks), preview.Scope)
	for index, task := range preview.Tasks {
		deps := ""
		if len(task.Dependencies) > 0 {
			deps = " ← " + strings.Join(task.Dependencies, ", ")
		}
		fmt.Fprintf(w, "%2d. %-28s %-14s %s%s\n", index+1, task.ID, task.Category, task.Name, deps)
	}
}

func runPlan(ctx context.Context, args []string, stdout, stderr io.Writer, env environment) int {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profileID := flags.String("profile", "", "profile ID")
	directory := flags.String("profiles", "", "external profile directory")
	selectValue := flags.String("select", "", "component IDs")
	asJSON := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *profileID == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "укажите --profile")
		return exitUsage
	}
	_, _, preview, err := buildPlan(*profileID, *directory, *selectValue, flags)
	if err != nil {
		fmt.Fprintf(stderr, "не удалось построить план: %v\n", err)
		return exitPlan
	}
	if ctx.Err() != nil {
		return exitPlan
	}
	if *asJSON {
		return writeJSON(stdout, preview)
	}
	printPreview(stdout, preview)
	return exitOK
}

func writeJSON(w io.Writer, value any) int {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return exitFailed
	}
	return exitOK
}

func runDeployment(ctx context.Context, args []string, stdout, stderr io.Writer, env environment) int {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profileID := flags.String("profile", "", "profile ID")
	directory := flags.String("profiles", "", "external profile directory")
	selectValue := flags.String("select", "", "component IDs")
	confirmed := flags.Bool("yes", false, "explicit confirmation of system changes")
	sessions := flags.String("sessions", env.Sessions, "session directory")
	reports := flags.String("reports", env.Reports, "report directory")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *profileID == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "укажите --profile")
		return exitUsage
	}
	profile, tasks, preview, err := buildPlan(*profileID, *directory, *selectValue, flags)
	if err != nil {
		fmt.Fprintf(stderr, "не удалось построить план: %v\n", err)
		return exitPlan
	}
	printPreview(stdout, preview)
	if !*confirmed {
		fmt.Fprintln(stderr, "выполнение изменяет систему; повторите с --yes")
		return exitConfirm
	}
	runner := deploy.Runner{Adapter: env.Adapter, ReadSystem: env.ReadSystem, Store: deploy.Store{Dir: *sessions}, ProfileID: profile.ID, ProfileName: profile.Name, System: env.System, Options: env.RunOptions, ReportDir: *reports}
	session, runErr := runner.Start(ctx, tasks)
	fmt.Fprintf(stdout, "Сессия: %s; отчёт: %s\n", session.ID, session.ReportHTML)
	if runErr != nil {
		fmt.Fprintf(stderr, "выполнение прервано: %v\n", runErr)
		return exitSession
	}
	return exitForSession(stdout, session)
}

func runResume(ctx context.Context, args []string, stdout, stderr io.Writer, env environment) int {
	flags := flag.NewFlagSet("resume", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sessionID := flags.String("session", "", "session ID")
	retryFailed := flags.Bool("retry-failed", false, "retry failed tasks")
	confirmed := flags.Bool("yes", false, "explicit confirmation of system changes")
	sessions := flags.String("sessions", env.Sessions, "session directory")
	reports := flags.String("reports", env.Reports, "report directory")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *sessionID == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "укажите --session")
		return exitUsage
	}
	if !*confirmed {
		fmt.Fprintln(stderr, "продолжение изменяет систему; повторите с --yes")
		return exitConfirm
	}
	store := deploy.Store{Dir: *sessions}
	loaded, err := store.Load(ctx, *sessionID)
	if err != nil {
		fmt.Fprintf(stderr, "сессия недоступна: %v\n", err)
		return exitSession
	}
	runner := deploy.Runner{Adapter: env.Adapter, ReadSystem: env.ReadSystem, Store: store, ProfileID: loaded.ProfileID, ProfileName: loaded.ProfileName, System: env.System, Options: env.RunOptions, ReportDir: *reports}
	session, runErr := runner.Resume(ctx, *sessionID, *retryFailed)
	fmt.Fprintf(stdout, "Сессия: %s; отчёт: %s\n", session.ID, session.ReportHTML)
	if runErr != nil {
		fmt.Fprintf(stderr, "продолжение прервано: %v\n", runErr)
		return exitSession
	}
	return exitForSession(stdout, session)
}

func runSessions(ctx context.Context, args []string, stdout, stderr io.Writer, env environment) int {
	flags := flag.NewFlagSet("sessions", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sessions := flags.String("sessions", env.Sessions, "session directory")
	asJSON := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	summaries, err := (deploy.Store{Dir: *sessions}).List(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "не удалось прочитать сессии: %v\n", err)
		return exitSession
	}
	if *asJSON {
		return writeJSON(stdout, summaries)
	}
	if len(summaries) == 0 {
		fmt.Fprintln(stdout, "Сессий нет.")
		return exitOK
	}
	for _, summary := range summaries {
		fmt.Fprintf(stdout, "%s  %-18s total=%d ok=%d failed=%d remaining=%d complete=%t\n", summary.ID, summary.ProfileName, summary.Total, summary.Succeeded, summary.Failed, summary.Remaining, summary.Completed)
	}
	return exitOK
}

func runReport(ctx context.Context, args []string, stdout, stderr io.Writer, env environment) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sessionID := flags.String("session", "", "session ID")
	sessions := flags.String("sessions", env.Sessions, "session directory")
	reports := flags.String("reports", env.Reports, "report directory")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *sessionID == "" {
		fmt.Fprintln(stderr, "укажите --session")
		return exitUsage
	}
	session, err := (deploy.Store{Dir: *sessions}).Load(ctx, *sessionID)
	if err != nil {
		fmt.Fprintf(stderr, "сессия недоступна: %v\n", err)
		return exitSession
	}
	report, err := reporting.Build(session.ID, session.ProfileID, session.ProfileName, session.System, session.Tasks, session.Snapshot.Results, session.StartedAt, session.FinishedAt, session.RebootRequired, session.Cancelled)
	if err != nil {
		fmt.Fprintf(stderr, "отчёт не сформирован: %v\n", err)
		return exitSession
	}
	jsonPath, htmlPath, err := reporting.Save(*reports, report)
	if err != nil {
		fmt.Fprintf(stderr, "отчёт не сохранён: %v\n", err)
		return exitSession
	}
	fmt.Fprintf(stdout, "JSON: %s\nHTML: %s\n", jsonPath, htmlPath)
	return exitOK
}

func exitForSession(stdout io.Writer, session deploy.Session) int {
	summary := session.Summarize()
	fmt.Fprintf(stdout, "Итог: успешно=%d, ошибок=%d, осталось=%d, перезагрузка=%t\n", summary.Succeeded, summary.Failed, summary.Remaining, session.RebootRequired)
	if summary.Failed > 0 || summary.Remaining > 0 || !session.Completed {
		return exitFailed
	}
	return exitOK
}

func loadCatalog() (packages.Catalog, error) {
	return profilecatalog.Load()
}
