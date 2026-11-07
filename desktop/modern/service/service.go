// Package service is the read-only application boundary used by Wails.
package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"windeploy/core/inventory"
	"windeploy/core/profiles"
)

type Entry struct {
	ID      uint64    `json:"id"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Source  string    `json:"source"`
	Message string    `json:"message"`
}
type State struct {
	SystemName      string            `json:"system_name,omitempty"`
	Scanning        bool              `json:"scanning"`
	CancelRequested bool              `json:"cancel_requested"`
	Generation      uint64            `json:"generation"`
	Status          string            `json:"status"`
	Report          *inventory.Report `json:"report"`
	Error           string            `json:"error"`
	Journal         []Entry           `json:"journal"`
}

// Reports are immutable once published. Native calls run outside the UI thread;
// cancellation is cooperative and a second scan cannot overlap an unfinished one.
type Service struct {
	profileSource fs.FS
	registry      map[string]profiles.Profile
	mu            sync.Mutex
	wg            sync.WaitGroup
	provider      inventory.Provider
	ctx           context.Context
	stop          context.CancelFunc
	cancel        context.CancelFunc
	closed        bool
	state         State
	nextEntry     uint64
	packages      *packageState
	deployment    *deploymentState
	operations    operationGate
}

func New(provider inventory.Provider) *Service {
	if provider == nil {
		provider = inventory.Native{}
	}
	ctx, stop := context.WithCancel(context.Background())
	s := &Service{provider: provider, ctx: ctx, stop: stop, state: State{Status: "idle", Journal: []Entry{}}}
	s.packages = newPackageState(ctx, defaultPackageDeps(), s.currentSystem)
	base := os.Getenv("WINDEPLOY_STATE_DIR")
	if base == "" {
		base = filepath.Join(os.TempDir(), "WinDeploy")
		if cache, err := os.UserCacheDir(); err == nil {
			base = filepath.Join(cache, "WinDeploy")
		}
	}
	s.deployment = newDeploymentState(ctx, base)
	s.appendLocked("info", "application", "WinDeploy запущен. Доступны просмотр сведений, установка программ и выполнение профилей.")
	return s
}

func (s *Service) appendLocked(level, source, message string) {
	s.nextEntry++
	s.state.Journal = append(s.state.Journal, Entry{ID: s.nextEntry, Time: time.Now().UTC(), Level: level, Source: source, Message: message})
	if len(s.state.Journal) > 200 {
		s.state.Journal = append([]Entry{}, s.state.Journal[len(s.state.Journal)-200:]...)
	}
}
func (s *Service) snapshotLocked() State {
	state := s.state
	if state.Report != nil && state.Report.System != nil {
		version := state.Report.System.Windows
		state.SystemName = map[string]string{"windows-11": "Windows 11", "windows-10": "Windows 10", "windows-7-sp1": "Windows 7 SP1"}[version.Token()]
		if state.SystemName == "" {
			state.SystemName = version.Label()
		}
	}
	state.Journal = append([]Entry{}, state.Journal...)
	return state
}
func (s *Service) GetState() State { s.mu.Lock(); defer s.mu.Unlock(); return s.snapshotLocked() }
func (s *Service) GetJournal() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Entry{}, s.state.Journal...)
}

func (s *Service) StartScan() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.snapshotLocked(), errors.New("Приложение завершает работу.")
	}
	if s.state.Scanning {
		return s.snapshotLocked(), nil
	}
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	s.cancel = cancel
	s.state.Scanning = true
	s.state.CancelRequested = false
	s.state.Generation++
	s.state.Status = "scanning"
	s.state.Error = ""
	s.appendLocked("info", "inventory", "Начато чтение сведений о компьютере.")
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		report, err := inventory.Collect(ctx, s.provider)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.state.Report = &report
		s.state.Scanning = false
		s.state.CancelRequested = false
		s.cancel = nil
		switch {
		case errors.Is(err, context.Canceled):
			s.state.Status = "cancelled"
			s.state.Error = "Чтение отменено. Показаны только успевшие поступить сведения."
		case errors.Is(err, context.DeadlineExceeded):
			s.state.Status = "partial"
			s.state.Error = "Время чтения истекло. Часть сведений недоступна."
		case errors.Is(err, inventory.ErrIncomplete):
			s.state.Status = "partial"
			s.state.Error = "Часть сведений недоступна. Проверьте статусы источников."
		case err != nil:
			s.state.Status = "error"
			s.state.Error = "Не удалось прочитать сведения о компьютере."
		default:
			s.state.Status = "complete"
			s.state.Error = ""
		}
		for _, section := range report.Sections {
			level := "info"
			if section.Status != "ok" {
				level = "warning"
			}
			s.appendLocked(level, section.Name, fmt.Sprintf("Источник %s: %s.", section.Name, section.Status))
		}
		if err == nil {
			s.appendLocked("info", "inventory", fmt.Sprintf("Чтение завершено: %d источников, %d записей программ.", len(report.Sections), len(report.Apps)))
		} else {
			s.appendLocked("warning", "inventory", s.state.Error)
		}
	}()
	return s.snapshotLocked(), nil
}

func (s *Service) CancelScan() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil && !s.state.CancelRequested {
		s.state.CancelRequested = true
		s.appendLocked("info", "inventory", "Запрошена отмена чтения. Ожидание завершения нативного вызова.")
		s.cancel()
	}
	return s.snapshotLocked()
}

// ServiceShutdown is the Wails lifecycle hook, excluded from generated bindings.
func (s *Service) ServiceShutdown() error {
	s.operations.close()
	s.mu.Lock()
	s.closed = true
	s.stop()
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
	if s.packages != nil {
		s.packages.shutdown()
	}
	if s.deployment != nil {
		s.deployment.shutdown()
	}
	s.wg.Wait()
	return nil
}
