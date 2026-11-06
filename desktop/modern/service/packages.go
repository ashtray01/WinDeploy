package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"windeploy/catalog"
	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/packages"
)

// PackageEnvironment is the read-only capability snapshot shown in the GUI.
type PackageEnvironment struct {
	Elevated      bool   `json:"elevated"`
	Winget        bool   `json:"winget"`
	WingetVersion string `json:"winget_version,omitempty"`
	Windows       string `json:"windows"`
	Architecture  string `json:"architecture"`
}

// PackageView is one row of the installable catalog with its current action.
type PackageView struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Publisher        string                `json:"publisher"`
	Category         string                `json:"category"`
	Group            string                `json:"group,omitempty"`
	Description      string                `json:"description,omitempty"`
	Requirements     engine.OSRequirements `json:"requirements"`
	RequiresAdmin    bool                  `json:"requires_admin"`
	Installed        bool                  `json:"installed"`
	InstalledVersion string                `json:"installed_version,omitempty"`
	Action           string                `json:"action"`
	ActionLabel      string                `json:"action_label"`
	Reason           string                `json:"reason"`
	SourceKind       string                `json:"source_kind,omitempty"`
	SourceVersion    string                `json:"source_version,omitempty"`
	Integrity        string                `json:"integrity,omitempty"`
	SizeBytes        int64                 `json:"size_bytes,omitempty"`
}

// PackageCatalogView is the complete package page state.
type PackageCatalogView struct {
	Source      string             `json:"source"`
	Environment PackageEnvironment `json:"environment"`
	Packages    []PackageView      `json:"packages"`
	Warnings    []string           `json:"warnings"`
}

// PackageEvent is a translated, sanitized progress message.
type PackageEvent struct {
	Time        time.Time `json:"time"`
	Level       string    `json:"level"`
	PackageID   string    `json:"package_id,omitempty"`
	PackageName string    `json:"package_name,omitempty"`
	Message     string    `json:"message"`
}

// PackageResultView is the final state of one package operation.
type PackageResultView struct {
	PackageID       string    `json:"package_id"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	StatusLabel     string    `json:"status_label"`
	Action          string    `json:"action"`
	SourceVersion   string    `json:"source_version,omitempty"`
	InstalledBefore string    `json:"installed_before,omitempty"`
	InstalledAfter  string    `json:"installed_after,omitempty"`
	ExitCode        int       `json:"exit_code,omitempty"`
	Attempts        int       `json:"attempts"`
	RebootRequired  bool      `json:"reboot_required"`
	ErrorCode       string    `json:"error_code,omitempty"`
	Error           string    `json:"error,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
}

// PackageInstallView is the live install progress observed by the UI.
type PackageInstallView struct {
	Running         bool                `json:"running"`
	CancelRequested bool                `json:"cancel_requested"`
	Generation      uint64              `json:"generation"`
	StartedAt       time.Time           `json:"started_at"`
	FinishedAt      time.Time           `json:"finished_at"`
	Total           int                 `json:"total"`
	Completed       int                 `json:"completed"`
	Current         string              `json:"current,omitempty"`
	CurrentName     string              `json:"current_name,omitempty"`
	Results         []PackageResultView `json:"results"`
	Events          []PackageEvent      `json:"events"`
	Message         string              `json:"message"`
	RebootRequired  bool                `json:"reboot_required"`
	Cancelled       bool                `json:"cancelled"`
}

type packageDeps struct {
	catalog    packages.Catalog
	catalogErr error
	facts      packages.Facts
	runner     packages.Runner
	signer     packages.Signer
	elevation  packages.Elevation
	winget     packages.WingetProbe
	fetcher    packages.Fetcher
	cacheDir   string
}

func defaultPackageDeps() packageDeps {
	loaded, loadErr := catalog.Load()
	facts, runner, signer, elevation, winget := packagePlatform()
	cacheDir, cacheErr := packageCacheDirectory()
	if loadErr == nil && cacheErr != nil {
		loadErr = cacheErr
	}
	return packageDeps{catalog: loaded, catalogErr: loadErr, facts: facts, runner: runner, signer: signer, elevation: elevation, winget: winget, fetcher: packages.NewHTTPFetcher(), cacheDir: cacheDir}
}

type packageState struct {
	mu       sync.Mutex
	ctx      context.Context
	stop     context.CancelFunc
	deps     packageDeps
	install  PackageInstallView
	cancel   context.CancelFunc
	closed   bool
	wg       sync.WaitGroup
	nextGen  uint64
	bindings map[string]string // package ID -> display name
	system   func(context.Context) inventory.SystemInfo
}

func newPackageState(parent context.Context, deps packageDeps, system func(context.Context) inventory.SystemInfo) *packageState {
	ctx, stop := context.WithCancel(parent)
	if deps.fetcher == nil {
		deps.fetcher = packages.NewHTTPFetcher()
	}
	state := &packageState{ctx: ctx, stop: stop, deps: deps, system: system, install: PackageInstallView{Results: []PackageResultView{}, Events: []PackageEvent{}}}
	state.bindings = map[string]string{}
	for _, pkg := range deps.catalog.Packages {
		state.bindings[pkg.ID] = pkg.Name
	}
	return state
}

func (p *packageState) shutdown() {
	p.mu.Lock()
	p.closed = true
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()
	p.stop()
	p.wg.Wait()
}

func (p *packageState) currentSystem(ctx context.Context) inventory.SystemInfo {
	if p.system == nil {
		return inventory.SystemInfo{}
	}
	return p.system(ctx)
}

func (p *packageState) manager(system inventory.SystemInfo, report func(packages.Event)) *packages.Manager {
	return &packages.Manager{
		Catalog:   p.deps.catalog,
		Facts:     p.deps.facts,
		System:    system,
		Winget:    p.deps.winget,
		Fetcher:   p.deps.fetcher,
		Runner:    p.deps.runner,
		Signer:    p.deps.signer,
		Elevation: p.deps.elevation,
		CacheDir:  p.deps.cacheDir,
		Report:    report,
	}
}

func (p *packageState) catalogView(ctx context.Context, system inventory.SystemInfo) (PackageCatalogView, error) {
	if p.deps.catalogErr != nil {
		return PackageCatalogView{}, errors.New("Каталог программ недоступен или содержит ошибку.")
	}
	planner := packages.Planner{Catalog: p.deps.catalog, Facts: p.deps.facts, System: system, Winget: p.deps.winget}
	plan, err := planner.Build(ctx, nil)
	if err != nil {
		return PackageCatalogView{}, errors.New("Не удалось проверить каталог программ.")
	}
	view := PackageCatalogView{Source: "Встроенный каталог WinDeploy", Packages: []PackageView{}, Warnings: []string{}}
	view.Environment = environment(ctx, system, p.deps.winget, p.deps.elevation)
	for _, warning := range plan.Warnings {
		if strings.Contains(warning, "unknown") {
			view.Warnings = append(view.Warnings, "Не удалось определить версию Windows и архитектуру: совместимость программ не подтверждена.")
			continue
		}
		view.Warnings = append(view.Warnings, warning)
	}
	for _, item := range plan.Items {
		view.Packages = append(view.Packages, PackageView{
			ID: item.PackageID, Name: item.Name, Publisher: item.Publisher, Category: item.Category,
			Group: item.Group, Description: item.Description, Requirements: item.Requirements,
			RequiresAdmin: item.RequiresAdmin, Installed: item.Installed.Installed,
			InstalledVersion: item.Installed.Version, Action: string(item.Action),
			ActionLabel: actionLabel(item.Action), Reason: reasonLabel(item.ReasonCode, item.Installed.Version),
			SourceKind: item.SourceKind, SourceVersion: item.SourceVersion,
			Integrity: item.Integrity, SizeBytes: item.SizeBytes,
		})
	}
	sortPackageViews(view.Packages)
	return view, nil
}

// GetPackages returns the catalog with detection state and a capability snapshot.
func (s *Service) GetPackages() (PackageCatalogView, error) {
	return s.packages.catalogView(context.Background(), s.packages.currentSystem(context.Background()))
}

// StartPackageInstall starts one background install run. Only one run may be
// active; progress is polled through GetPackageInstall.
func (s *Service) StartPackageInstall(ids []string) (PackageInstallView, error) {
	p := s.packages
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return p.install, errors.New("Приложение завершает работу.")
	}
	if p.deps.catalogErr != nil {
		return p.install, errors.New("Каталог программ недоступен.")
	}
	if p.install.Running {
		return p.install, errors.New("Установка уже выполняется.")
	}
	if len(ids) == 0 {
		return p.install, errors.New("Выберите хотя бы одну программу.")
	}
	if err := p.deps.catalog.ValidateSelection(ids); err != nil {
		return p.install, errors.New("Выбран неизвестный пакет.")
	}
	if p.deps.elevation != nil {
		elevated, elevateErr := p.deps.elevation.Elevated(context.Background())
		if elevateErr != nil {
			return p.install, errors.New("Не удалось определить права текущего процесса.")
		}
		if !elevated {
			for _, id := range ids {
				if pkg, ok := p.deps.catalog.Find(id); ok && pkg.RequiresAdmin {
					return p.install, errors.New("Для установки выбранных программ нужны права администратора. Перезапустите WinDeploy от имени администратора.")
				}
			}
		}
	}
	p.nextGen++
	ctx, cancel := context.WithCancel(p.ctx)
	p.cancel = cancel
	p.install = PackageInstallView{
		Running: true, Generation: p.nextGen, StartedAt: time.Now().UTC(), Total: len(ids),
		Results: make([]PackageResultView, 0, len(ids)), Events: []PackageEvent{}, Message: "Запуск установки.",
	}
	for _, id := range ids {
		p.install.Results = append(p.install.Results, PackageResultView{PackageID: id, Name: p.bindings[id], Status: "queued", StatusLabel: "В очереди", Attempts: 0})
	}
	idsCopy := append([]string{}, ids...)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.run(ctx, idsCopy)
	}()
	return p.install, nil
}

func (p *packageState) run(ctx context.Context, ids []string) {
	system := p.currentSystem(ctx)
	manager := p.manager(system, p.recordEvent)
	run, err := manager.Install(ctx, ids, packages.InstallOptions{MaxAttempts: 3})
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancel = nil
	p.install.Running = false
	p.install.CancelRequested = false
	p.install.FinishedAt = time.Now().UTC()
	if err != nil {
		p.install.Message = installErrorLabel(err)
		if errors.Is(err, packages.ErrElevationRequired) {
			p.install.Results = markResultsElevation(p.install.Results)
		}
		return
	}
	p.install.Results = make([]PackageResultView, 0, len(run.Results))
	succeeded, failed := 0, 0
	for _, result := range run.Results {
		view := resultView(result)
		p.install.Results = append(p.install.Results, view)
		switch result.Status {
		case packages.StatusSuccess, packages.StatusRebootRequired, packages.StatusUpToDate:
			succeeded++
		case packages.StatusFailed:
			failed++
		}
	}
	p.install.Completed = len(run.Results)
	p.install.RebootRequired = run.RebootRequired
	p.install.Cancelled = run.Cancelled
	switch {
	case run.Cancelled:
		p.install.Message = "Установка отменена. Уже запущенные установщики остановлены."
	case run.RebootRequired:
		p.install.Message = fmt.Sprintf("Установка завершена. Требуется перезагрузка. Успешно: %d, ошибок: %d.", succeeded, failed)
	case failed > 0:
		p.install.Message = fmt.Sprintf("Установка завершена с ошибками. Успешно: %d, ошибок: %d.", succeeded, failed)
	default:
		p.install.Message = fmt.Sprintf("Установка завершена. Успешно: %d.", succeeded)
	}
	p.appendEventLocked("info", "", "", p.install.Message)
}

func (p *packageState) recordEvent(event packages.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	level, message := translateEvent(event)
	if message == "" {
		return
	}
	p.install.Current = event.PackageID
	p.install.CurrentName = p.bindings[event.PackageID]
	p.appendEventLocked(level, event.PackageID, p.install.CurrentName, message)
}

func (p *packageState) appendEventLocked(level, packageID, name, message string) {
	if len(message) > 400 {
		message = message[:400]
	}
	p.install.Events = append(p.install.Events, PackageEvent{Time: time.Now().UTC(), Level: level, PackageID: packageID, PackageName: name, Message: sanitizeMessage(message)})
	if len(p.install.Events) > 300 {
		p.install.Events = append([]PackageEvent{}, p.install.Events[len(p.install.Events)-300:]...)
	}
}

func sanitizeMessage(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) {
			continue
		}
		builder.WriteRune(r)
		if builder.Len() >= 400 {
			break
		}
	}
	return strings.TrimSpace(builder.String())
}

// GetPackageInstall returns a copy of the live install progress.
func (s *Service) GetPackageInstall() PackageInstallView {
	p := s.packages
	p.mu.Lock()
	defer p.mu.Unlock()
	return copyInstallView(p.install)
}

// CancelPackageInstall requests cooperative cancellation of the active run.
func (s *Service) CancelPackageInstall() PackageInstallView {
	p := s.packages
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.install.Running {
		return copyInstallView(p.install)
	}
	if p.cancel != nil && !p.install.CancelRequested {
		p.install.CancelRequested = true
		p.appendEventLocked("warning", p.install.Current, p.install.CurrentName, "Запрошена отмена установки. Ожидание завершения установщика.")
		p.cancel()
	}
	return copyInstallView(p.install)
}

func copyInstallView(view PackageInstallView) PackageInstallView {
	view.Results = append([]PackageResultView{}, view.Results...)
	view.Events = append([]PackageEvent{}, view.Events...)
	return view
}

func (s *Service) currentSystem(ctx context.Context) inventory.SystemInfo {
	s.mu.Lock()
	if s.state.Report != nil && s.state.Report.System != nil {
		system := *s.state.Report.System
		s.mu.Unlock()
		return system
	}
	s.mu.Unlock()
	if s.provider != nil {
		if system, err := s.provider.System(ctx); err == nil {
			return system
		}
	}
	return inventory.SystemInfo{}
}

func environment(ctx context.Context, system inventory.SystemInfo, winget packages.WingetProbe, elevation packages.Elevation) PackageEnvironment {
	env := PackageEnvironment{Windows: "Неизвестно", Architecture: system.Architecture}
	switch system.Windows.Token() {
	case "windows-11":
		env.Windows = "Windows 11"
	case "windows-10":
		env.Windows = "Windows 10"
	case "windows-7-sp1":
		env.Windows = "Windows 7 SP1"
	}
	if winget != nil {
		env.Winget, env.WingetVersion = winget.Available(ctx)
	}
	if elevation != nil {
		env.Elevated, _ = elevation.Elevated(ctx)
	}
	return env
}

func actionLabel(action packages.Action) string {
	switch action {
	case packages.ActionInstall:
		return "Установить"
	case packages.ActionUpdate:
		return "Обновить"
	case packages.ActionUpToDate:
		return "Уже установлено"
	case packages.ActionUnavailable:
		return "Нет источника"
	case packages.ActionIncompatible:
		return "Несовместимо"
	default:
		return "Не определено"
	}
}

func reasonLabel(code, installedVersion string) string {
	switch code {
	case "not-installed":
		return "Программа не установлена."
	case "update-available":
		return "Доступна более новая версия из каталога."
	case "up-to-date":
		return "Установленная версия актуальна" + versionSuffix(installedVersion) + "."
	case "installed":
		return "Обнаружена установленная программа" + versionSuffix(installedVersion) + "."
	case "os-incompatible":
		return "Требования ОС или архитектуры не совпадают."
	case "os-unknown":
		return "Не удалось определить версию Windows и архитектуру."
	case "winget-unavailable":
		return "WinGet недоступен, а локальный установщик не найден."
	case "file-source-missing":
		return "Локальный установщик не найден в каталоге пакетов."
	case "no-source":
		return "Не найден источник, подходящий этой системе."
	case "detection-unavailable":
		return "Не удалось прочитать список установленных программ."
	default:
		return "Состояние не определено."
	}
}

func versionSuffix(version string) string {
	if version == "" {
		return ""
	}
	return " " + version
}

func resultView(result packages.Result) PackageResultView {
	errorText := ""
	switch result.ErrorCode {
	case packages.CodeElevationRequired:
		errorText = "Требуются права администратора."
	case packages.CodeHashMismatch:
		errorText = "SHA-256 загруженного файла не совпадает."
	case packages.CodeSignatureFailed:
		errorText = "Подпись издателя недействительна."
	case packages.CodeDownloadFailed:
		errorText = "Не удалось загрузить установщик."
	case packages.CodeSourceUnavailable:
		errorText = "Источник установки недоступен."
	case packages.CodeVerifyFailed:
		errorText = "Установка не подтверждена проверкой."
	case packages.CodeInstallFailed:
		errorText = "Установщик завершился с ошибкой."
	case packages.CodeCancelled:
		errorText = "Операция отменена."
	}
	if result.Error != "" && errorText == "" {
		errorText = "Операция завершилась с ошибкой."
	}
	return PackageResultView{
		PackageID: result.PackageID, Name: result.Name, Status: string(result.Status),
		StatusLabel: statusLabel(result.Status), Action: string(result.Action),
		SourceVersion: result.SourceVersion, InstalledBefore: result.InstalledBefore,
		InstalledAfter: result.InstalledAfter, ExitCode: result.ExitCode, Attempts: result.Attempts,
		RebootRequired: result.RebootRequired, ErrorCode: result.ErrorCode, Error: errorText,
		StartedAt: result.StartedAt, FinishedAt: result.FinishedAt,
	}
}

func statusLabel(status packages.Status) string {
	switch status {
	case packages.StatusSuccess:
		return "Установлено"
	case packages.StatusRebootRequired:
		return "Установлено, нужна перезагрузка"
	case packages.StatusUpToDate:
		return "Уже установлено"
	case packages.StatusSkipped:
		return "Пропущено"
	case packages.StatusCancelled:
		return "Отменено"
	case packages.StatusUnavailable:
		return "Нет источника"
	case packages.StatusIncompatible:
		return "Несовместимо"
	case packages.StatusUnknown:
		return "Не определено"
	case packages.StatusFailed:
		return "Ошибка"
	default:
		return "В очереди"
	}
}

func installErrorLabel(err error) string {
	if errors.Is(err, packages.ErrElevationRequired) {
		return "Для установки выбранных программ нужны права администратора. Перезапустите WinDeploy от имени администратора."
	}
	return "Не удалось запустить установку. Проверьте журнал и повторите попытку."
}

func markResultsElevation(results []PackageResultView) []PackageResultView {
	for i := range results {
		results[i].Status = "failed"
		results[i].StatusLabel = "Нужны права администратора"
		results[i].ErrorCode = packages.CodeElevationRequired
		results[i].Error = "Требуются права администратора."
	}
	return results
}

func translateEvent(event packages.Event) (string, string) {
	detail := sanitizeMessage(event.Detail)
	if len(detail) > 120 {
		detail = detail[:120]
	}
	switch event.Code {
	case "package.already-installed":
		return "info", "Уже установлено" + detailSuffix(detail, " ") + "."
	case "package.source-unavailable":
		return "warning", "Нет доступного источника установки."
	case "package.incompatible":
		return "warning", "Программа несовместима с этой версией Windows."
	case "package.unknown":
		return "warning", "Не удалось определить состояние программы."
	case "package.winget-unavailable":
		return "warning", "WinGet недоступен."
	case "package.download-start":
		return "info", "Загрузка установщика…"
	case "package.download-complete":
		return "info", "Загрузка завершена: " + detail + "."
	case "package.download-failed":
		return "warning", "Ошибка загрузки, повторная попытка."
	case "package.cache-hit":
		return "info", "Используется ранее проверенный файл из кэша."
	case "package.hash-verified":
		return "info", "SHA-256 установщика подтверждён."
	case "package.hash-mismatch":
		return "error", "SHA-256 не совпадает. Установка остановлена."
	case "package.signature-verified":
		message := "Подпись издателя подтверждена."
		if detail != "" {
			message = "Подпись издателя подтверждена: " + detail + "."
		}
		return "info", message
	case "package.signature-failed":
		return "error", "Подпись издателя недействительна. Установка остановлена."
	case "package.signature-unavailable":
		return "warning", "Проверка подписи издателя недоступна."
	case "package.publisher-mismatch":
		return "error", "Издатель не совпадает с каталогом. Установка остановлена."
	case "package.install-start":
		return "info", "Запуск установщика."
	case "package.install-success":
		return "info", "Установка завершена."
	case "package.install-retry":
		return "warning", "Установщик занят, повторная попытка через несколько секунд."
	case "package.reboot-required":
		return "warning", "Установка выполнена. Требуется перезагрузка Windows."
	case "package.verify-failed":
		return "error", "Программа не обнаружена после установки."
	case "package.verified":
		message := "Установка подтверждена."
		if detail != "" {
			message = "Установка подтверждена, версия " + detail + "."
		}
		return "info", message
	case "package.portable-installed":
		return "info", "Программа установлена."
	case "package.elevation-required":
		return "error", "Требуются права администратора."
	default:
		return "", ""
	}
}

func detailSuffix(detail, prefix string) string {
	if detail == "" {
		return ""
	}
	return prefix + detail
}

// SortPackagesByCategory keeps a stable order for the UI.
func sortPackageViews(items []PackageView) {
	rank := map[string]int{"browsers": 0, "messengers": 1, "media": 2, "utilities": 3, "printing": 4}
	sort.SliceStable(items, func(i, j int) bool {
		left, ok := rank[items[i].Category]
		if !ok {
			left = 99
		}
		right, ok := rank[items[j].Category]
		if !ok {
			right = 99
		}
		return left < right
	})
}
