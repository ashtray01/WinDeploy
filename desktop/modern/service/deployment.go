package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"windeploy/core/browsers"
	"windeploy/core/deploy"
	"windeploy/core/drivers"
	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/network"
	"windeploy/core/packages"
	"windeploy/core/printers"
	coreprofiles "windeploy/core/profiles"
	"windeploy/core/settings"
	"windeploy/core/shares"
)

// DeployTaskView is one task in the deployment plan.
type DeployTaskView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Status      string    `json:"status"`
	StatusLabel string    `json:"status_label"`
	Attempts    int       `json:"attempts"`
	Reboot      bool      `json:"reboot_required"`
	Error       string    `json:"error,omitempty"`
	Warnings    []string  `json:"warnings"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	FinishedAt  time.Time `json:"finished_at,omitempty"`
}

// DeployEvent is a translated progress record.
type DeployEvent struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// DeploymentView is the live execution state.
type DeploymentView struct {
	Running         bool             `json:"running"`
	CancelRequested bool             `json:"cancel_requested"`
	Generation      uint64           `json:"generation"`
	SessionID       string           `json:"session_id"`
	ProfileID       string           `json:"profile_id"`
	ProfileName     string           `json:"profile_name"`
	StartedAt       time.Time        `json:"started_at"`
	FinishedAt      time.Time        `json:"finished_at"`
	Tasks           []DeployTaskView `json:"tasks"`
	Events          []DeployEvent    `json:"events"`
	Message         string           `json:"message"`
	RebootRequired  bool             `json:"reboot_required"`
	Cancelled       bool             `json:"cancelled"`
	Completed       bool             `json:"completed"`
	ReportJSON      string           `json:"report_json,omitempty"`
	ReportHTML      string           `json:"report_html,omitempty"`
}

// DeploySessionView is one saved session summary.
type DeploySessionView struct {
	ID             string    `json:"id"`
	ProfileID      string    `json:"profile_id"`
	ProfileName    string    `json:"profile_name"`
	UpdatedAt      time.Time `json:"updated_at"`
	Total          int       `json:"total"`
	Succeeded      int       `json:"succeeded"`
	Failed         int       `json:"failed"`
	Remaining      int       `json:"remaining"`
	Completed      bool      `json:"completed"`
	Cancelled      bool      `json:"cancelled"`
	RebootRequired bool      `json:"reboot_required"`
}

type deploymentState struct {
	mu      sync.Mutex
	ctx     context.Context
	stop    context.CancelFunc
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	store   deploy.Store
	reports string
	running bool
	closed  bool
	nextGen uint64
	view    DeploymentView
	seen    map[string]bool
}

func newDeploymentState(parent context.Context, base string) *deploymentState {
	ctx, stop := context.WithCancel(parent)
	return &deploymentState{ctx: ctx, stop: stop, store: deploy.Store{Dir: filepath.Join(base, "sessions")}, reports: filepath.Join(base, "reports"), seen: map[string]bool{}}
}

func (d *deploymentState) shutdown() {
	d.mu.Lock()
	d.closed = true
	if d.cancel != nil {
		d.cancel()
	}
	d.mu.Unlock()
	d.stop()
	d.wg.Wait()
}

// deploymentAdapter wires the shared core managers for task execution.
func (s *Service) deploymentAdapter(system inventory.SystemInfo) (*deploy.Adapter, error) {
	if s.packages == nil || s.packages.deps.catalogErr != nil {
		return nil, errors.New("Каталог программ недоступен.")
	}
	deps := s.packages.deps
	packageManager := &packages.Manager{
		Catalog: deps.catalog, Facts: deps.facts, System: system, Winget: deps.winget,
		Fetcher: deps.fetcher, Runner: packages.ContextRunner{Next: deps.runner, Check: s.requireUserContext}, Signer: deps.signer, Elevation: deps.elevation, CacheDir: deps.cacheDir,
	}
	shareStore, shareConnector, shareShortcuts := sharePlatform()
	desktop, _ := desktopDirectory()
	browserDesktop, _ := browsers.DesktopDirectory()
	adapter := &deploy.Adapter{
		CheckUserContext: s.requireUserContext,
		PackageManager:   packageManager,
		Shares:           &shares.Manager{Store: shareStore, Connector: shareConnector, Shortcuts: shareShortcuts},
		Drivers:          &drivers.Manager{Devices: drivers.WindowsDevices{}, Installer: drivers.WindowsInstaller{Runner: drivers.WindowsRunner{}, System: system}, Verifier: drivers.WindowsINFVerifier{}, Elevation: drivers.WindowsElevation{}},
		Printers:         &printers.Manager{Printers: printers.WindowsPrinters{}, Installer: printers.WindowsInstaller{}, Defaults: printers.WindowsDefaults{}, Tester: printers.WindowsTestPrinter{}, Elevation: printers.WindowsElevation{}},
		Browsers:         &browsers.Manager{Store: browsers.NewRegistryStore(), Shortcuts: browsers.WindowsShortcut{}, Locator: browsers.WindowsLocator{}, Elevation: browsers.WindowsElevation{}, DesktopDir: browserDesktop},
		Settings:         &settings.Manager{Registry: settings.WindowsRegistry{}, Runner: settings.WindowsRunner{}, Services: settings.WindowsServices{}, Elevation: settings.WindowsElevation{}, Activation: settings.WindowsActivation{}, Computer: settings.WindowsComputerName{}, System: system},
		NetworkDeps:      network.NativeDependencies(),
		NetworkConfig:    func(ctx context.Context) ([]inventory.AdapterConfig, error) { return s.provider.NetworkConfig(ctx) },
		ShortcutDir:      desktop,
		System:           system,
	}
	return adapter, nil
}

// StartDeployment executes the selected profile components in the background.
func (s *Service) StartDeployment(profileID string, selected []string) (DeploymentView, error) {
	release, err := s.operations.acquire()
	if err != nil {
		return DeploymentView{}, err
	}
	started := false
	defer func() {
		if !started {
			release()
		}
	}()
	d := s.deployment
	d.mu.Lock()
	if d.closed || d.ctx.Err() != nil {
		d.mu.Unlock()
		return DeploymentView{}, errors.New("Приложение завершает работу.")
	}
	if d.running {
		view := d.view
		d.mu.Unlock()
		return view, errors.New("Развёртывание уже выполняется.")
	}
	d.mu.Unlock()
	profile, tasks, err := s.deploymentTasks(profileID, selected)
	if err != nil {
		return DeploymentView{}, err
	}
	if len(tasks) == 0 {
		return DeploymentView{}, errors.New("План не содержит задач.")
	}
	system, err := s.provider.System(d.ctx)
	if err != nil {
		return DeploymentView{}, errors.New("Не удалось проверить систему перед выполнением.")
	}
	adapter, err := s.deploymentAdapter(system)
	if err != nil {
		return DeploymentView{}, err
	}
	d.mu.Lock()
	if d.closed || d.ctx.Err() != nil {
		d.mu.Unlock()
		return DeploymentView{}, errors.New("Приложение завершает работу.")
	}
	if d.running {
		d.mu.Unlock()
		return DeploymentView{}, errors.New("Развёртывание уже выполняется.")
	}
	d.nextGen++
	ctx, cancel := context.WithCancel(d.ctx)
	d.cancel = cancel
	d.running = true
	d.seen = map[string]bool{}
	taskViews := make([]DeployTaskView, 0, len(tasks))
	for _, task := range tasks {
		taskViews = append(taskViews, DeployTaskView{ID: task.ID, Name: task.Name, Category: task.Category, Status: string(engine.Pending), StatusLabel: statusLabelDeploy(engine.Pending), Warnings: []string{}})
	}
	sessionID := deploy.NewSessionID()
	d.view = DeploymentView{Running: true, Generation: d.nextGen, SessionID: sessionID, ProfileID: profile.ID, ProfileName: profile.Name, StartedAt: time.Now().UTC(), Tasks: taskViews, Events: []DeployEvent{}, Message: "Выполнение запущено."}
	view := d.view
	d.wg.Add(1)
	started = true
	go func() {
		defer d.wg.Done()
		defer release()
		runner := deploy.Runner{Adapter: adapter, Store: d.store, ProfileID: profile.ID, ProfileName: profile.Name, ReadSystem: s.provider.System, CheckUserContext: s.requireUserContext, Options: engine.RunOptions{MaxAttempts: 2, RetryDelay: 2 * time.Second, MaxParallel: 1}, ReportDir: d.reports, SessionID: sessionID}
		session, runErr := runner.Start(ctx, tasks)
		d.finish(session, runErr)
	}()
	d.mu.Unlock()
	return view, nil
}

// ResumeDeployment continues a saved session after a reboot or interruption.
func (s *Service) ResumeDeployment(sessionID string, retryFailed bool) (DeploymentView, error) {
	release, err := s.operations.acquire()
	if err != nil {
		return DeploymentView{}, err
	}
	started := false
	defer func() {
		if !started {
			release()
		}
	}()
	d := s.deployment
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return DeploymentView{}, errors.New("Приложение завершает работу.")
	}
	if d.running {
		d.mu.Unlock()
		return DeploymentView{}, errors.New("Развёртывание уже выполняется.")
	}
	d.mu.Unlock()
	session, err := d.store.Load(context.Background(), sessionID)
	if err != nil {
		return DeploymentView{}, errors.New("Сессия недоступна.")
	}
	if session.Completed && !retryFailed {
		return DeploymentView{}, errors.New("Сессия уже завершена. Для повтора ошибок выберите повтор неудачных задач.")
	}
	system, err := s.provider.System(d.ctx)
	if err != nil {
		return DeploymentView{}, errors.New("Не удалось проверить систему перед продолжением.")
	}
	adapter, err := s.deploymentAdapter(system)
	if err != nil {
		return DeploymentView{}, err
	}
	d.mu.Lock()
	if d.closed || d.ctx.Err() != nil || d.running {
		d.mu.Unlock()
		return DeploymentView{}, errors.New("Развёртывание недоступно: операция уже выполняется или приложение завершает работу.")
	}
	d.nextGen++
	ctx, cancel := context.WithCancel(d.ctx)
	d.cancel = cancel
	d.running = true
	d.seen = map[string]bool{}
	taskViews := make([]DeployTaskView, 0, len(session.Tasks))
	known := map[string]engine.TaskResult{}
	for _, result := range session.Snapshot.Results {
		known[result.TaskID] = result
	}
	for _, task := range session.Tasks {
		result, ok := known[task.ID]
		status := engine.Pending
		if ok {
			status = result.Status
		}
		taskViews = append(taskViews, DeployTaskView{ID: task.ID, Name: task.Name, Category: task.Category, Status: string(status), StatusLabel: statusLabelDeploy(status), Attempts: result.Attempts, Reboot: result.RebootRequired, Error: result.Error, Warnings: append([]string{}, result.Warnings...), StartedAt: result.StartedAt, FinishedAt: result.FinishedAt})
	}
	d.view = DeploymentView{Running: true, Generation: d.nextGen, SessionID: session.ID, ProfileID: session.ProfileID, ProfileName: session.ProfileName, StartedAt: time.Now().UTC(), Tasks: taskViews, Events: []DeployEvent{}, Message: "Продолжение сессии."}
	view := d.view
	d.wg.Add(1)
	started = true
	go func() {
		defer d.wg.Done()
		defer release()
		runner := deploy.Runner{Adapter: adapter, Store: d.store, ProfileID: session.ProfileID, ProfileName: session.ProfileName, ReadSystem: s.provider.System, CheckUserContext: s.requireUserContext, Options: engine.RunOptions{MaxAttempts: 2, RetryDelay: 2 * time.Second, MaxParallel: 1}, ReportDir: d.reports}
		resumed, runErr := runner.Resume(ctx, session.ID, retryFailed)
		d.finish(resumed, runErr)
	}()
	d.mu.Unlock()
	return view, nil
}

func (s *Service) deploymentTasks(profileID string, selected []string) (coreprofiles.Profile, []engine.Task, error) {
	s.mu.Lock()
	registry := s.registry
	s.mu.Unlock()
	if registry == nil {
		if _, err := s.ListProfiles(); err != nil {
			return coreprofiles.Profile{}, nil, errors.New("Каталог профилей недоступен.")
		}
		s.mu.Lock()
		registry = s.registry
		s.mu.Unlock()
	}
	profile, err := coreprofiles.Resolve(registry, profileID)
	if err != nil {
		return coreprofiles.Profile{}, nil, errors.New("Профиль не найден или содержит ошибку.")
	}
	tasks, err := coreprofiles.BuildPlan(profile, selected)
	if err != nil {
		return coreprofiles.Profile{}, nil, errors.New("Не удалось построить план: проверьте выбор и зависимости.")
	}
	return profile, tasks, nil
}

func (d *deploymentState) finish(session deploy.Session, runErr error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
	d.cancel = nil
	d.view.Running = false
	d.view.FinishedAt = time.Now().UTC()
	if session.ID != "" {
		d.view.SessionID = session.ID
		d.view.ProfileID = session.ProfileID
		d.view.ProfileName = session.ProfileName
		d.view.ReportJSON, d.view.ReportHTML = session.ReportJSON, session.ReportHTML
		d.view.RebootRequired = session.RebootRequired
		d.view.Cancelled = session.Cancelled
		d.view.Completed = session.Completed && runErr == nil
		known := map[string]engine.TaskResult{}
		for _, result := range session.Snapshot.Results {
			known[result.TaskID] = result
		}
		for index := range d.view.Tasks {
			if result, ok := known[d.view.Tasks[index].ID]; ok {
				d.view.Tasks[index] = deployTaskView(result, d.view.Tasks[index].Name, d.view.Tasks[index].Category)
			}
		}
		if runErr == nil {
			d.appendEventLocked("info", fmt.Sprintf("Выполнение завершено. Успешно: %d, ошибок: %d.", session.Summarize().Succeeded, session.Summarize().Failed))
		}
	}
	switch {
	case errors.Is(runErr, engine.ErrRebootPending):
		d.view.Message = "Ожидается перезагрузка Windows. Продолжение в текущей загрузке заблокировано."
		d.appendEventLocked("warning", d.view.Message)
	case errors.Is(runErr, context.Canceled), errors.Is(runErr, context.DeadlineExceeded):
		d.view.Cancelled = true
		d.view.Message = "Выполнение отменено. Завершённые задачи сохранены."
		d.appendEventLocked("warning", d.view.Message)
	case runErr != nil:
		d.view.Message = "Выполнение прервано. Проверьте журнал и отчёт."
		d.appendEventLocked("error", d.view.Message)
	case session.RebootRequired && !session.Completed:
		d.view.Message = "Результат ещё не проверен. Перезагрузите Windows и продолжите сессию для проверки."
		d.appendEventLocked("warning", d.view.Message)
	default:
		d.view.Message = "Выполнение завершено."
	}
}

// GetDeployment returns the live view, refreshing progress from the session file.
func (s *Service) GetDeployment() DeploymentView {
	d := s.deployment
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running && d.view.SessionID != "" {
		if session, err := d.store.Load(context.Background(), d.view.SessionID); err == nil {
			known := map[string]engine.TaskResult{}
			for _, result := range session.Snapshot.Results {
				known[result.TaskID] = result
			}
			for index := range d.view.Tasks {
				if result, ok := known[d.view.Tasks[index].ID]; ok {
					previous := d.view.Tasks[index].Status
					d.view.Tasks[index] = deployTaskView(result, d.view.Tasks[index].Name, d.view.Tasks[index].Category)
					if result.Status != engine.Running && previous != string(result.Status) {
						d.appendEventLocked("info", fmt.Sprintf("%s: %s", d.view.Tasks[index].Name, statusLabelDeploy(result.Status)))
					}
				}
			}
		}
	}
	return copyDeployment(d.view)
}

// CancelDeployment requests cooperative cancellation.
func (s *Service) CancelDeployment() DeploymentView {
	d := s.deployment
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running && d.cancel != nil && !d.view.CancelRequested {
		d.view.CancelRequested = true
		d.appendEventLocked("warning", "Запрошена отмена развёртывания.")
		d.cancel()
	}
	return copyDeployment(d.view)
}

// ListDeploymentSessions lists saved sessions newest first.
func (s *Service) ListDeploymentSessions() ([]DeploySessionView, error) {
	summaries, err := s.deployment.store.List(context.Background())
	if err != nil {
		return nil, errors.New("Не удалось прочитать сессии развёртывания.")
	}
	views := make([]DeploySessionView, 0, len(summaries))
	for _, summary := range summaries {
		views = append(views, DeploySessionView{ID: summary.ID, ProfileID: summary.ProfileID, ProfileName: summary.ProfileName, UpdatedAt: summary.UpdatedAt, Total: summary.Total, Succeeded: summary.Succeeded, Failed: summary.Failed, Remaining: summary.Remaining, Completed: summary.Completed, Cancelled: summary.Cancelled, RebootRequired: summary.RebootRequired})
	}
	return views, nil
}

// GetDeploymentSession returns a saved session's task details for review.
func (s *Service) GetDeploymentSession(sessionID string) (DeploymentView, error) {
	session, err := s.deployment.store.Load(context.Background(), sessionID)
	if err != nil {
		return DeploymentView{}, errors.New("Сессия недоступна.")
	}
	view := DeploymentView{Running: false, SessionID: session.ID, ProfileID: session.ProfileID, ProfileName: session.ProfileName, StartedAt: session.StartedAt, FinishedAt: session.FinishedAt, RebootRequired: session.RebootRequired, Cancelled: session.Cancelled, Completed: session.Completed, ReportJSON: session.ReportJSON, ReportHTML: session.ReportHTML, Message: "Сохранённая сессия.", Events: []DeployEvent{}}
	known := map[string]engine.TaskResult{}
	for _, result := range session.Snapshot.Results {
		known[result.TaskID] = result
	}
	for _, task := range session.Tasks {
		item := DeployTaskView{ID: task.ID, Name: task.Name, Category: task.Category, Status: string(engine.Pending), StatusLabel: statusLabelDeploy(engine.Pending), Warnings: []string{}}
		if result, ok := known[task.ID]; ok {
			item = deployTaskView(result, task.Name, task.Category)
		}
		view.Tasks = append(view.Tasks, item)
	}
	return view, nil
}

func (d *deploymentState) appendEventLocked(level, message string) {
	d.view.Events = append(d.view.Events, DeployEvent{Time: time.Now().UTC(), Level: level, Message: sanitizeMessage(message)})
	if len(d.view.Events) > 200 {
		d.view.Events = append([]DeployEvent{}, d.view.Events[len(d.view.Events)-200:]...)
	}
}

func deployTaskView(result engine.TaskResult, name, category string) DeployTaskView {
	warnings := append([]string{}, result.Warnings...)
	if warnings == nil {
		warnings = []string{}
	}
	return DeployTaskView{ID: result.TaskID, Name: name, Category: category, Status: string(result.Status), StatusLabel: statusLabelDeploy(result.Status), Attempts: result.Attempts, Reboot: result.RebootRequired, Error: translateTaskError(result.Error), Warnings: warnings, StartedAt: result.StartedAt, FinishedAt: result.FinishedAt}
}

func statusLabelDeploy(status engine.TaskStatus) string {
	switch status {
	case engine.Pending:
		return "В очереди"
	case engine.Running:
		return "Выполняется"
	case engine.Success:
		return "Успешно"
	case engine.AlreadyInstalled:
		return "Уже установлено"
	case engine.NotApplicable:
		return "Не применимо"
	case engine.Skipped:
		return "Пропущено"
	case engine.Failed:
		return "Ошибка"
	case engine.Cancelled:
		return "Отменено"
	case engine.RebootRequired:
		return "Ожидает проверки после перезагрузки"
	default:
		return string(status)
	}
}

func translateTaskError(value string) string {
	switch value {
	case "precheck failed":
		return "Ошибка предварительной проверки"
	case "execute failed":
		return "Ошибка выполнения"
	case "verify failed":
		return "Не удалось подтвердить результат"
	case "task deadline exceeded":
		return "Превышено время задачи"
	case "plan cancelled":
		return "План отменён"
	case "dependency did not succeed":
		return "Зависимость завершилась ошибкой"
	default:
		return ""
	}
}

func copyDeployment(view DeploymentView) DeploymentView {
	view.Tasks = append([]DeployTaskView{}, view.Tasks...)
	view.Events = append([]DeployEvent{}, view.Events...)
	return view
}
