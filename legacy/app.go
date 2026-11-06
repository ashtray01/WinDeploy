package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lxn/walk"
	profilecatalog "windeploy/catalog"
	"windeploy/core/inventory"
	"windeploy/core/network"
	"windeploy/core/packages"
	"windeploy/core/settings"
	"windeploy/core/shares"
)

// legacyApp is the native Windows 7-compatible UI. All long operations run in
// goroutines and update the UI through MainWindow.Synchronize.
type legacyApp struct {
	window      *walk.MainWindow
	summary     *walk.TextEdit
	logView     *walk.TextEdit
	tabs        *walk.TabWidget
	programList *walk.Composite
	uiReady     atomic.Bool
	packageBox  map[string]*walk.CheckBox
	packageInfo map[string]packages.Package

	catalog  packages.Catalog
	system   inventory.SystemInfo
	cpu      inventory.CPUInfo
	memory   inventory.MemoryInfo
	adapters []inventory.AdapterConfig
	admin    bool

	shareUNC      *walk.LineEdit
	shareName     *walk.LineEdit
	shareUser     *walk.LineEdit
	sharePassword *walk.LineEdit
	shareShortcut *walk.CheckBox
	installButton *walk.PushButton
	cancelButton  *walk.PushButton

	installMu     sync.Mutex
	installCancel context.CancelFunc
	installing    bool

	stateMu sync.RWMutex

	logMu  sync.Mutex
	logBuf strings.Builder
}

func (a *legacyApp) stateCopy() (inventory.SystemInfo, inventory.CPUInfo, inventory.MemoryInfo, []inventory.AdapterConfig, bool) {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return a.system, a.cpu, a.memory, append([]inventory.AdapterConfig{}, a.adapters...), a.admin
}

func newLegacyApp() (*legacyApp, error) {
	catalog, err := profilecatalog.Load()
	if err != nil {
		return nil, fmt.Errorf("каталог программ недоступен")
	}
	app := &legacyApp{catalog: catalog, packageBox: map[string]*walk.CheckBox{}, packageInfo: map[string]packages.Package{}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	native := inventory.Native{}
	app.system, _ = native.System(ctx)
	app.cpu, _ = native.CPU(ctx)
	app.memory, _ = native.Memory(ctx)
	app.adapters, _ = native.NetworkConfig(ctx)
	if elevated, err := (packages.WindowsElevation{}).Elevated(ctx); err == nil {
		app.admin = elevated
	}
	if err := app.buildWindow(); err != nil {
		return nil, err
	}
	app.refreshSummary()
	app.buildPackages()
	go app.detectPackages()
	return app, nil
}

func (a *legacyApp) buildWindow() error {
	window, err := walk.NewMainWindow()
	if err != nil {
		return err
	}
	a.window = window
	window.SetTitle("WinDeploy Legacy — подготовка рабочих станций")
	window.SetSize(walk.Size{Width: 1040, Height: 780})
	window.SetMinMaxSize(walk.Size{Width: 860, Height: 620}, walk.Size{})
	layout := walk.NewVBoxLayout()
	window.SetLayout(layout)

	tabs, err := walk.NewTabWidget(window)
	if err != nil {
		return err
	}
	a.tabs = tabs
	_ = layout.SetStretchFactor(tabs, 1)

	computerPage, err := walk.NewTabPage()
	if err != nil {
		return err
	}
	computerPage.SetTitle("Компьютер")
	computerPage.SetLayout(walk.NewVBoxLayout())
	if err := tabs.Pages().Add(computerPage); err != nil {
		return err
	}
	summary, err := walk.NewTextEdit(computerPage)
	if err != nil {
		return err
	}
	a.summary = summary
	summary.SetReadOnly(true)
	_ = summary.SetMinMaxSize(walk.Size{Height: 260}, walk.Size{})
	summary.SetText("Чтение сведений о компьютере…")
	actions, err := walk.NewComposite(computerPage)
	if err != nil {
		return err
	}
	actions.SetLayout(walk.NewHBoxLayout())
	refresh, _ := walk.NewPushButton(actions)
	refresh.SetText("Обновить сведения")
	refresh.Clicked().Attach(func() { go a.refreshSummaryAsync() })
	update, _ := walk.NewPushButton(actions)
	update.SetText("Диагностика Windows Update")
	update.Clicked().Attach(func() { go a.windowsUpdateDiagnostics() })
	activation, _ := walk.NewPushButton(actions)
	activation.SetText("Статус лицензирования")
	activation.Clicked().Attach(func() { go a.showActivation() })
	quit, _ := walk.NewPushButton(actions)
	quit.SetText("Выход")
	quit.Clicked().Attach(func() { a.window.Close() })

	programsPage, err := walk.NewTabPage()
	if err != nil {
		return err
	}
	programsPage.SetTitle("Программы")
	programsPage.SetLayout(walk.NewVBoxLayout())
	if err := tabs.Pages().Add(programsPage); err != nil {
		return err
	}
	hint, _ := walk.NewLabel(programsPage)
	hint.SetText("Отметьте программы для установки. Уже установленные и несовместимые программы недоступны.")
	programList, err := walk.NewComposite(programsPage)
	if err != nil {
		return err
	}
	programList.SetLayout(walk.NewVBoxLayout())
	a.programList = programList
	programActions, _ := walk.NewComposite(programsPage)
	programActions.SetLayout(walk.NewHBoxLayout())
	install, _ := walk.NewPushButton(programActions)
	install.SetText("Установить выбранные")
	a.installButton = install
	install.Clicked().Attach(func() { go a.installSelected() })
	cancelInstall, _ := walk.NewPushButton(programActions)
	cancelInstall.SetText("Отменить установку")
	cancelInstall.SetEnabled(false)
	a.cancelButton = cancelInstall
	cancelInstall.Clicked().Attach(func() { a.cancelInstallation() })
	recheck, _ := walk.NewPushButton(programActions)
	recheck.SetText("Проверить установленные")
	recheck.Clicked().Attach(func() { go a.detectPackages() })
	adminLabel, _ := walk.NewLabel(programActions)
	if a.admin {
		adminLabel.SetText("Права администратора: есть")
	} else {
		adminLabel.SetText("Права администратора: нет (системные установщики будут отклонены)")
	}

	networkPage, err := walk.NewTabPage()
	if err != nil {
		return err
	}
	networkPage.SetTitle("Сеть и папки")
	networkPage.SetLayout(walk.NewVBoxLayout())
	if err := tabs.Pages().Add(networkPage); err != nil {
		return err
	}
	networkHint, _ := walk.NewLabel(networkPage)
	networkHint.SetText("Диагностика только читает состояние. Подключение сетевой папки меняет систему после подтверждения.")
	diagnose, _ := walk.NewPushButton(networkPage)
	diagnose.SetText("Диагностика сети")
	diagnose.Clicked().Attach(func() { go a.runNetworkDiagnostics() })
	shareGroup, _ := walk.NewGroupBox(networkPage)
	shareGroup.SetTitle("Подключить сетевую папку")
	shareGroup.SetLayout(walk.NewVBoxLayout())
	shareForm, _ := walk.NewComposite(shareGroup)
	shareForm.SetLayout(walk.NewGridLayout())
	a.shareUNC = addRow(shareForm, 0, "UNC-путь", `\\server\share`)
	a.shareName = addRow(shareForm, 1, "Название", "")
	a.shareUser = addRow(shareForm, 2, "Пользователь", `DOMAIN\user`)
	a.sharePassword = addRow(shareForm, 3, "Пароль", "")
	a.sharePassword.SetPasswordMode(true)
	a.shareShortcut, _ = walk.NewCheckBox(shareGroup)
	a.shareShortcut.SetText("Создать ярлык на рабочем столе")
	connect, _ := walk.NewPushButton(shareGroup)
	connect.SetText("Подключить папку")
	connect.Clicked().Attach(func() { go a.connectShare() })

	logLabel, _ := walk.NewLabel(window)
	logLabel.SetText("Журнал")
	logView, err := walk.NewTextEdit(window)
	if err != nil {
		return err
	}
	a.logView = logView
	logView.SetReadOnly(true)
	_ = logView.SetMinMaxSize(walk.Size{Height: 170}, walk.Size{})
	_ = layout.SetStretchFactor(logView, 0)

	window.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if a.installing {
			*canceled = true
			walk.MsgBox(window, "WinDeploy Legacy", "Дождитесь завершения установки или отмените её.", walk.MsgBoxIconInformation)
			return
		}
	})
	return nil
}

func addRow(parent walk.Container, row int, label, placeholder string) *walk.LineEdit {
	caption, _ := walk.NewLabel(parent)
	caption.SetText(label)
	if layout, ok := parent.Layout().(*walk.GridLayout); ok {
		_ = layout.SetRange(caption, walk.Rectangle{X: 0, Y: row, Width: 1, Height: 1})
	}
	edit, _ := walk.NewLineEdit(parent)
	if placeholder != "" {
		edit.SetText(placeholder)
	}
	if layout, ok := parent.Layout().(*walk.GridLayout); ok {
		_ = layout.SetRange(edit, walk.Rectangle{X: 1, Y: row, Width: 1, Height: 1})
	}
	return edit
}

func (a *legacyApp) buildPackages() {
	target := a.programList
	if target == nil {
		return
	}
	for _, pkg := range a.catalog.Packages {
		check, err := walk.NewCheckBox(target)
		if err != nil {
			continue
		}
		check.SetText(packageLabel(pkg, packages.InstalledState{}))
		if !packageCompatible(pkg, a.system) {
			check.SetEnabled(false)
			check.SetText(check.Text() + " — несовместимо с этой Windows")
		}
		a.packageBox[pkg.ID] = check
		a.packageInfo[pkg.ID] = pkg
	}
	a.appendLog("Каталог программ загружен: " + fmt.Sprint(len(a.catalog.Packages)))
}

func packageCompatible(pkg packages.Package, system inventory.SystemInfo) bool {
	ok, err := inventory.Applicable(pkg.Requirements, system)
	return err == nil && ok
}

func packageLabel(pkg packages.Package, state packages.InstalledState) string {
	if state.Installed {
		if state.Version != "" {
			return pkg.Name + " — установлено " + state.Version
		}
		return pkg.Name + " — установлено"
	}
	return pkg.Name + " — " + pkg.Publisher
}

func (a *legacyApp) detectPackages() {
	system, _, _, _, _ := a.stateCopy()
	facts := packages.WindowsFacts{}
	for id, pkg := range a.packageInfo {
		if !packageCompatible(pkg, system) {
			continue
		}
		state, _ := packages.Detect(context.Background(), facts, pkg)
		check := a.packageBox[id]
		if check == nil {
			continue
		}
		label := packageLabel(pkg, state)
		installed := state.Installed
		a.window.Synchronize(func() {
			check.SetText(label)
			check.SetChecked(!installed)
			check.SetEnabled(!installed)
		})
	}
	a.appendLog("Проверка установленных программ завершена.")
}

func (a *legacyApp) selectedPackages() []string {
	result := []string{}
	for id, check := range a.packageBox {
		if check.Enabled() && check.Checked() {
			result = append(result, id)
		}
	}
	return result
}

func (a *legacyApp) installSelected() {
	selected := a.selectedPackages()
	if len(selected) == 0 {
		walk.MsgBox(a.window, "WinDeploy Legacy", "Не выбрано ни одной программы.", walk.MsgBoxIconInformation)
		return
	}
	if walk.MsgBox(a.window, "Подтверждение установки", fmt.Sprintf("Установить выбранные программы (%d)? Операция изменит систему.", len(selected)), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	system, _, _, _, admin := a.stateCopy()
	if !admin {
		walk.MsgBox(a.window, "Нужны права администратора", "WinDeploy Legacy запущен без прав администратора. Перезапустите приложение от имени администратора.", walk.MsgBoxIconWarning)
		return
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		a.appendLog("Не удалось определить каталог загрузки.")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.installMu.Lock()
	a.installCancel = cancel
	a.installing = true
	a.installMu.Unlock()
	a.window.Synchronize(func() {
		a.installButton.SetEnabled(false)
		a.cancelButton.SetEnabled(true)
	})
	manager := &packages.Manager{
		Catalog: a.catalog, Facts: packages.WindowsFacts{}, System: system, Winget: &packages.WindowsWinget{},
		Fetcher: packages.NewHTTPFetcher(), Runner: packages.WindowsRunner{}, Signer: packages.WindowsSigner{},
		Elevation: packages.WindowsElevation{}, CacheDir: filepath.Join(cache, "WinDeploy", "Downloads"),
		Report: func(event packages.Event) {
			a.appendLogEvent(event)
		},
	}
	run, installErr := manager.Install(ctx, selected, packages.InstallOptions{MaxAttempts: 2})
	cancel()
	a.installMu.Lock()
	a.installing = false
	a.installCancel = nil
	a.installMu.Unlock()
	a.window.Synchronize(func() {
		a.installButton.SetEnabled(true)
		a.cancelButton.SetEnabled(false)
	})
	if installErr != nil {
		a.appendLog("Установка прервана: " + installErr.Error())
		walk.MsgBox(a.window, "WinDeploy Legacy", "Установка не выполнена. Подробности в журнале.", walk.MsgBoxIconWarning)
	} else {
		failed := 0
		for _, result := range run.Results {
			if result.Status == packages.StatusFailed {
				failed++
			}
		}
		a.appendLog(fmt.Sprintf("Установка завершена: успешно=%d, ошибок=%d, перезагрузка=%t", len(run.Results)-failed, failed, run.RebootRequired))
		if run.RebootRequired {
			walk.MsgBox(a.window, "Требуется перезагрузка", "Установка выполнена. Перезагрузите компьютер для завершения.", walk.MsgBoxIconInformation)
		}
	}
	go a.detectPackages()
}

func (a *legacyApp) cancelInstallation() {
	a.installMu.Lock()
	defer a.installMu.Unlock()
	if a.installCancel != nil {
		a.installCancel()
		a.appendLog("Запрошена отмена установки.")
	}
}

func (a *legacyApp) summaryText() string {
	activation := "неизвестно"
	manager := &settings.Manager{Activation: settings.WindowsActivation{}}
	if status, err := manager.ActivationStatus(context.Background()); err == nil {
		activation = status.Status
	}
	system, cpu, memory, _, admin := a.stateCopy()
	return formatSummary(system.Hostname, windowsFriendly(system.Windows), system.Architecture, cpu.Model, cpu.LogicalProcessors, memory.TotalPhysicalBytes, admin, activation)
}

func (a *legacyApp) refreshSummary() {
	a.summary.SetText(a.summaryText())
}

func (a *legacyApp) refreshSummaryAsync() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	native := inventory.Native{}
	system, systemErr := native.System(ctx)
	cpu, cpuErr := native.CPU(ctx)
	memory, memoryErr := native.Memory(ctx)
	adapters, adaptersErr := native.NetworkConfig(ctx)
	admin := a.admin
	if elevated, err := (packages.WindowsElevation{}).Elevated(ctx); err == nil {
		admin = elevated
	}
	a.stateMu.Lock()
	if systemErr == nil {
		a.system = system
	}
	if cpuErr == nil {
		a.cpu = cpu
	}
	if memoryErr == nil {
		a.memory = memory
	}
	if adaptersErr == nil {
		a.adapters = adapters
	}
	a.admin = admin
	a.stateMu.Unlock()
	text := a.summaryText()
	a.window.Synchronize(func() { a.summary.SetText(text) })
	a.appendLog("Сведения о компьютере обновлены.")
	go a.detectPackages()
}

func (a *legacyApp) windowsUpdateDiagnostics() {
	system, _, _, _, _ := a.stateCopy()
	manager := &settings.Manager{Registry: settings.WindowsRegistry{}, Runner: settings.WindowsRunner{}, Services: settings.WindowsServices{}, Elevation: settings.WindowsElevation{}, Activation: settings.WindowsActivation{}, Computer: settings.WindowsComputerName{}, System: system}
	report, err := manager.UpdateDiagnostics(context.Background())
	if err != nil {
		a.appendLog("Диагностика Windows Update недоступна.")
		return
	}
	for _, check := range report.Checks {
		a.appendLog(fmt.Sprintf("[%s] %s: %s", check.Status, check.Name, check.Detail))
	}
}

func (a *legacyApp) showActivation() {
	manager := &settings.Manager{Activation: settings.WindowsActivation{}}
	status, err := manager.ActivationStatus(context.Background())
	if err != nil {
		a.appendLog("Статус лицензирования недоступен.")
		return
	}
	a.appendLog(fmt.Sprintf("Лицензирование: %s — %s", status.Status, status.Detail))
}

func (a *legacyApp) runNetworkDiagnostics() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _, _, adapters, _ := a.stateCopy()
	plan := network.Plan{Adapters: adapters, Timeout: 4 * time.Second}
	for _, adapter := range adapters {
		if adapter.Operational {
			plan.Gateways = append(plan.Gateways, adapter.Gateways...)
		}
	}
	plan.DNSNames = append(plan.DNSNames, "localhost")
	checks := network.BuildChecks(plan)
	report, err := network.Diagnose(ctx, checks, network.NativeDependencies())
	if err != nil {
		a.appendLog("Сетевая диагностика прервана: " + err.Error())
		return
	}
	for _, result := range report.Results {
		a.appendLog(fmt.Sprintf("[%s] %s — %s", result.Status, result.Name, result.Detail))
	}
	a.appendLog(fmt.Sprintf("Сетевая диагностика завершена: ошибок %d из %d.", report.Failed, len(report.Results)))
}

func (a *legacyApp) connectShare() {
	share, err := shareFromForm(a.shareUNC.Text(), a.shareName.Text(), a.shareUser.Text(), a.shareShortcut.Checked())
	if err != nil {
		walk.MsgBox(a.window, "WinDeploy Legacy", "Проверьте UNC-путь вида \\\\сервер\\ресурс.", walk.MsgBoxIconWarning)
		return
	}
	if walk.MsgBox(a.window, "Подключение сетевой папки", "Подключить "+share.UNC+"?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	desktop, _ := shares.DesktopDirectory()
	manager := &shares.Manager{Store: shares.WindowsCredentialStore{}, Connector: shares.WindowsConnector{}, Shortcuts: shares.WindowsShortcut{}}
	result, err := manager.Add(context.Background(), share, shares.Options{Password: a.sharePassword.Text(), ShortcutDir: desktop})
	a.sharePassword.SetText("")
	if err != nil {
		a.appendLog("Не удалось подключить сетевую папку: " + err.Error())
		walk.MsgBox(a.window, "WinDeploy Legacy", "Подключение не выполнено. Проверьте путь, сервер и учётные данные.", walk.MsgBoxIconWarning)
		return
	}
	a.appendLog(fmt.Sprintf("Сетевая папка: %s; ярлык: %s", result.UNC, result.ShortcutPath))
}

func (a *legacyApp) appendLog(message string) {
	a.logMu.Lock()
	a.logBuf.WriteString(time.Now().Format("15:04:05") + "  " + strings.TrimSpace(message) + "\r\n")
	text := a.logBuf.String()
	if len(text) > 60000 {
		text = text[len(text)-60000:]
		a.logBuf.Reset()
		a.logBuf.WriteString(text)
	}
	a.logMu.Unlock()
	if a.window != nil && a.logView != nil {
		update := func() {
			_ = a.logView.SetText(text)
			a.logView.SetTextSelection(len(text), len(text))
		}
		if a.uiReady.Load() {
			a.window.Synchronize(update)
		} else {
			update()
		}
	}
}

func (a *legacyApp) appendLogEvent(event packages.Event) {
	switch event.Code {
	case "package.download-start":
		a.appendLog(event.PackageID + ": загрузка установщика")
	case "package.download-complete":
		a.appendLog(event.PackageID + ": загрузка завершена")
	case "package.cache-hit":
		a.appendLog(event.PackageID + ": проверенный кэш")
	case "package.hash-verified":
		a.appendLog(event.PackageID + ": SHA-256 подтверждён")
	case "package.signature-verified":
		a.appendLog(event.PackageID + ": подпись издателя подтверждена")
	case "package.install-start":
		a.appendLog(event.PackageID + ": запуск установщика")
	case "package.install-success", "package.verified":
		a.appendLog(event.PackageID + ": установка подтверждена")
	case "package.reboot-required":
		a.appendLog(event.PackageID + ": требуется перезагрузка")
	case "package.already-installed":
		a.appendLog(event.PackageID + ": уже установлено")
	case "package.hash-mismatch":
		a.appendLog(event.PackageID + ": SHA-256 не совпадает")
	case "package.signature-failed":
		a.appendLog(event.PackageID + ": подпись недействительна")
	case "package.install-retry":
		a.appendLog(event.PackageID + ": повтор установки")
	case "package.verify-failed":
		a.appendLog(event.PackageID + ": установка не подтверждена")
	case "package.elevation-required":
		a.appendLog(event.PackageID + ": нужны права администратора")
	}
}
