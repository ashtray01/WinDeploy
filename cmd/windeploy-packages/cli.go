package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	pkgcatalog "windeploy/catalog"
	"windeploy/core/inventory"
	"windeploy/core/packages"
	"windeploy/core/usercontext"
)

type cliAdapters struct {
	facts            packages.Facts
	fetcher          packages.Fetcher
	runner           packages.Runner
	signer           packages.Signer
	elevation        packages.Elevation
	winget           packages.WingetProbe
	cacheDir         string
	system           inventory.SystemInfo
	systemErr        error
	checkUserContext func(context.Context) error
}

const (
	exitOK        = 0
	exitUsage     = 2
	exitCatalog   = 3
	exitPlan      = 4
	exitFailed    = 5
	exitElevation = 6
	exitConfirm   = 7
)

func run(args []string, stdout, stderr io.Writer, adapters cliAdapters) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "list":
		return runList(args[1:], stdout, stderr, adapters)
	case "plan":
		return runPlan(args[1:], stdout, stderr, adapters)
	case "download":
		return runDownload(args[1:], stdout, stderr, adapters)
	case "install":
		return runInstall(args[1:], stdout, stderr, adapters)
	case "help", "-h", "--help":
		usage(stdout)
		return exitOK
	default:
		fmt.Fprintf(stderr, "Неизвестная команда %q.\n\n", args[0])
		usage(stderr)
		return exitUsage
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `WinDeploy Package Manager (Windows)

  windeploy-packages list   [-catalog FILE] [-json]
  windeploy-packages plan   [-catalog FILE] [-id a,b] [-no-winget] [-json]
  windeploy-packages download [-catalog FILE] [-id a,b] [-cache DIR] [-no-winget] [-json]
  windeploy-packages install [-catalog FILE] [-id a,b] [-yes] [-dry-run] [-no-winget] [-json]

Команда install изменяет систему и требует прав администратора и явного --yes.
Без -id выбираются все программы каталога. Каталог по умолчанию встроен в EXE.
-no-winget проверяет локальные и HTTPS-источники так, как это будет на машине без WinGet.
`)
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func loadCatalog(path string) (packages.Catalog, error) {
	if path == "" {
		return pkgcatalog.Load()
	}
	file, err := os.Open(path)
	if err != nil {
		return packages.Catalog{}, fmt.Errorf("каталог недоступен")
	}
	defer file.Close()
	return packages.LoadCatalog(file)
}

func splitIDs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func newManager(catalog packages.Catalog, adapters cliAdapters) *packages.Manager {
	check := adapters.checkUserContext
	if check == nil {
		check = usercontext.Native{}.Check
	}
	return &packages.Manager{
		Catalog: catalog, Facts: adapters.facts, System: adapters.system, Winget: adapters.winget,
		Fetcher: adapters.fetcher, Runner: packages.ContextRunner{Next: adapters.runner, Check: check}, Signer: adapters.signer,
		Elevation: adapters.elevation, CacheDir: adapters.cacheDir,
		Report: func(event packages.Event) {
			if event.Level == "warning" || event.Level == "error" {
				fmt.Fprintf(os.Stderr, "  %s: %s\n", event.PackageID, event.Code)
			}
		},
	}
}

func actionLabel(action packages.Action) string {
	switch action {
	case packages.ActionInstall:
		return "установить"
	case packages.ActionUpdate:
		return "обновить"
	case packages.ActionUpToDate:
		return "актуально"
	case packages.ActionUnavailable:
		return "нет источника"
	case packages.ActionIncompatible:
		return "несовместимо"
	default:
		return "неизвестно"
	}
}

func printPlan(w io.Writer, plan packages.Plan) {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ПАКЕТ\tДЕЙСТВИЕ\tВЕРСИЯ\tУСТАНОВЛЕНО\tИСТОЧНИК\tПРИЧИНА")
	for _, item := range plan.Items {
		installed := item.Installed.Version
		if installed == "" {
			installed = "—"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n", item.PackageID, actionLabel(item.Action), emptyDash(item.SourceVersion), installed, emptyDash(item.SourceKind), item.ReasonCode)
	}
	_ = table.Flush()
}

func printInstallRun(w io.Writer, run packages.Run) {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ПАКЕТ\tРЕЗУЛЬТАТ\tВЕРСИЯ\tКОД\tПЕРЕЗАГРУЗКА\tОШИБКА")
	for _, result := range run.Results {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n", result.PackageID, result.Status, emptyDash(result.InstalledAfter), exitCode(result.ExitCode), yesNo(result.RebootRequired), emptyDash(result.ErrorCode))
	}
	_ = table.Flush()
}

func exitCode(code int) string {
	if code == 0 {
		return "—"
	}
	return fmt.Sprintf("%d", code)
}

func yesNo(value bool) string {
	if value {
		return "да"
	}
	return "нет"
}

func emptyDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func writeJSON(w io.Writer, value any) int {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return exitFailed
	}
	return exitOK
}

func runList(args []string, stdout, stderr io.Writer, adapters cliAdapters) int {
	flags := newFlagSet("list", stderr)
	catalogPath := flags.String("catalog", "", "внешний YAML-каталог")
	asJSON := flags.Bool("json", false, "вывод в JSON")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	catalog, err := loadCatalog(*catalogPath)
	if err != nil {
		fmt.Fprintf(stderr, "Ошибка каталога: %v.\n", err)
		return exitCatalog
	}
	type entry struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Publisher  string `json:"publisher"`
		Category   string `json:"category"`
		Sources    int    `json:"sources"`
		Admin      bool   `json:"requires_admin"`
		Compatible bool   `json:"compatible"`
	}
	entries := make([]entry, 0, len(catalog.Packages))
	for _, pkg := range catalog.Packages {
		compatible, applicableErr := inventory.Applicable(pkg.Requirements, adapters.system)
		entries = append(entries, entry{ID: pkg.ID, Name: pkg.Name, Publisher: pkg.Publisher, Category: pkg.Category, Sources: len(pkg.Sources), Admin: pkg.RequiresAdmin, Compatible: applicableErr == nil && compatible})
	}
	if *asJSON {
		return writeJSON(stdout, entries)
	}
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tНАЗВАНИЕ\tИЗДАТЕЛЬ\tКАТЕГОРИЯ\tИСТОЧНИКИ\tПРАВА")
	for _, item := range entries {
		rights := "пользователь"
		if item.Admin {
			rights = "администратор"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\t%s\n", item.ID, item.Name, item.Publisher, item.Category, item.Sources, rights)
	}
	_ = table.Flush()
	return exitOK
}

func runPlan(args []string, stdout, stderr io.Writer, adapters cliAdapters) int {
	flags := newFlagSet("plan", stderr)
	catalogPath := flags.String("catalog", "", "внешний YAML-каталог")
	ids := flags.String("id", "", "ID пакетов через запятую")
	asJSON := flags.Bool("json", false, "вывод в JSON")
	noWinget := flags.Bool("no-winget", false, "игнорировать WinGet")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *noWinget {
		adapters.winget = nil
	}
	catalog, err := loadCatalog(*catalogPath)
	if err != nil {
		fmt.Fprintf(stderr, "Ошибка каталога: %v.\n", err)
		return exitCatalog
	}
	if adapters.systemErr != nil {
		fmt.Fprintln(stderr, "Предупреждение: не удалось прочитать сведения о Windows; совместимость не подтверждена.")
	}
	plan, err := (packages.Planner{Catalog: catalog, Facts: adapters.facts, System: adapters.system, Winget: adapters.winget}).Build(context.Background(), splitIDs(*ids))
	if err != nil {
		fmt.Fprintf(stderr, "Не удалось построить план: %v.\n", err)
		return exitPlan
	}
	if *asJSON {
		return writeJSON(stdout, plan)
	}
	printPlan(stdout, plan)
	return exitOK
}

func runDownload(args []string, stdout, stderr io.Writer, adapters cliAdapters) int {
	flags := newFlagSet("download", stderr)
	catalogPath := flags.String("catalog", "", "внешний YAML-каталог")
	ids := flags.String("id", "", "ID пакетов через запятую")
	cacheDir := flags.String("cache", "", "каталог кэша установщиков")
	asJSON := flags.Bool("json", false, "вывод в JSON")
	noWinget := flags.Bool("no-winget", false, "игнорировать WinGet")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *noWinget {
		adapters.winget = nil
	}
	catalog, err := loadCatalog(*catalogPath)
	if err != nil {
		fmt.Fprintf(stderr, "Ошибка каталога: %v.\n", err)
		return exitCatalog
	}
	if *cacheDir != "" {
		adapters.cacheDir = *cacheDir
	}
	if adapters.cacheDir == "" {
		fmt.Fprintln(stderr, "Не удалось определить каталог кэша; укажите -cache.")
		return exitUsage
	}
	manager := newManager(catalog, adapters)
	run, err := manager.Download(context.Background(), splitIDs(*ids), packages.InstallOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "Не удалось подготовить установщики: %v.\n", err)
		return exitPlan
	}
	if *asJSON {
		return writeJSON(stdout, run)
	}
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ПАКЕТ\tРЕЗУЛЬТАТ\tРАЗМЕР\tSHA-256\tКЭШ")
	for _, result := range run.Results {
		digest := result.SHA256
		if len(digest) > 16 {
			digest = digest[:16] + "…"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", result.PackageID, result.Status, formatSize(result.Size), emptyDash(digest), yesNo(result.Cached))
	}
	_ = table.Flush()
	for _, result := range run.Results {
		if result.Status == packages.StatusFailed || result.Status == packages.StatusCancelled {
			return exitFailed
		}
	}
	return exitOK
}

func runInstall(args []string, stdout, stderr io.Writer, adapters cliAdapters) int {
	flags := newFlagSet("install", stderr)
	catalogPath := flags.String("catalog", "", "внешний YAML-каталог")
	ids := flags.String("id", "", "ID пакетов через запятую")
	cacheDir := flags.String("cache", "", "каталог кэша установщиков")
	asJSON := flags.Bool("json", false, "вывод в JSON")
	confirmed := flags.Bool("yes", false, "явное подтверждение изменения системы")
	dryRun := flags.Bool("dry-run", false, "проверить план без установки")
	noWinget := flags.Bool("no-winget", false, "игнорировать WinGet")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *noWinget {
		adapters.winget = nil
	}
	if !*confirmed && !*dryRun {
		fmt.Fprintln(stderr, "Установка изменяет систему. Повторите с явным подтверждением --yes.")
		return exitConfirm
	}
	if !*dryRun {
		check := adapters.checkUserContext
		if check == nil {
			check = usercontext.Native{}.Check
		}
		if err := check(context.Background()); err != nil {
			fmt.Fprintln(stderr, "Пользовательский контекст не подтверждён. Запустите WinDeploy под владельцем рабочего стола.")
			return exitPlan
		}
	}
	catalog, err := loadCatalog(*catalogPath)
	if err != nil {
		fmt.Fprintf(stderr, "Ошибка каталога: %v.\n", err)
		return exitCatalog
	}
	if *cacheDir != "" {
		adapters.cacheDir = *cacheDir
	}
	manager := newManager(catalog, adapters)
	run, err := manager.Install(context.Background(), splitIDs(*ids), packages.InstallOptions{DryRun: *dryRun, MaxAttempts: 3})
	if err != nil {
		if errors.Is(err, packages.ErrElevationRequired) {
			fmt.Fprintln(stderr, "Для установки выбранных программ нужны права администратора.")
			return exitElevation
		}
		fmt.Fprintf(stderr, "Не удалось запустить установку: %v.\n", err)
		return exitPlan
	}
	if *asJSON {
		return writeJSON(stdout, run)
	}
	printInstallRun(stdout, run)
	if run.Cancelled {
		return exitFailed
	}
	for _, result := range run.Results {
		if result.Status == packages.StatusFailed {
			return exitFailed
		}
	}
	return exitOK
}

func formatSize(size int64) string {
	if size <= 0 {
		return "—"
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%d КБ", size/1024)
	}
	return fmt.Sprintf("%.1f МБ", float64(size)/(1024*1024))
}
