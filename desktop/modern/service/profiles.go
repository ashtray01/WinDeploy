package service

import (
	"errors"
	"io/fs"
	"sort"
	"strings"

	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/profiles"
	profileassets "windeploy/profiles"
)

type ProfileSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Parent string `json:"parent"`
}
type Catalog struct {
	Source   string           `json:"source"`
	Profiles []ProfileSummary `json:"profiles"`
	Errors   []string         `json:"errors"`
}
type ProfileDetails struct {
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Components []engine.PreviewTask `json:"components"`
}
type PlannedTask struct {
	Task          engine.PreviewTask `json:"task"`
	Compatibility string             `json:"compatibility"`
	Reason        string             `json:"reason"`
}
type ProfilePreview struct {
	Scope string        `json:"scope"`
	Tasks []PlannedTask `json:"tasks"`
}

// NewWithProfiles permits a caller-selected, read-only profile directory.
func NewWithProfiles(provider inventory.Provider, source fs.FS) *Service {
	s := New(provider)
	s.profileSource = source
	return s
}

func (s *Service) ListProfiles() (Catalog, error) {
	s.mu.Lock()
	s.registry = nil // A failed reload must never leave an old catalog usable.
	s.mu.Unlock()
	source := s.profileSource
	label := "Внешний каталог YAML"
	if source == nil {
		source = profileassets.Files
		label = "Встроенные YAML-профили"
	}
	catalog := Catalog{Source: label, Profiles: []ProfileSummary{}, Errors: []string{}}
	files, err := fs.ReadDir(source, ".")
	if err != nil {
		return catalog, errors.New("Каталог профилей недоступен.")
	}
	registry := map[string]profiles.Profile{}
	for _, file := range files {
		if file.IsDir() || !(strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml")) {
			continue
		}
		if len(catalog.Profiles)+len(catalog.Errors) >= 128 {
			return catalog, errors.New("Слишком много профилей в каталоге.")
		}
		f, openErr := source.Open(file.Name())
		if openErr != nil {
			catalog.Errors = append(catalog.Errors, file.Name()+": файл недоступен")
			continue
		}
		p, loadErr := profiles.Load(f)
		f.Close()
		if loadErr != nil {
			catalog.Errors = append(catalog.Errors, file.Name()+": недействительный YAML-профиль")
			continue
		}
		if _, duplicate := registry[p.ID]; duplicate {
			return catalog, errors.New("Повторяющиеся ID профилей. Каталог не загружен.")
		}
		registry[p.ID] = p
		catalog.Profiles = append(catalog.Profiles, ProfileSummary{p.ID, p.Name, p.Extends})
	}
	sort.Slice(catalog.Profiles, func(i, j int) bool { return catalog.Profiles[i].Name < catalog.Profiles[j].Name })
	s.mu.Lock()
	s.registry = registry
	s.appendLocked("info", "profiles", "Прочитан каталог YAML-профилей.")
	s.mu.Unlock()
	return catalog, nil
}
func (s *Service) resolvedProfile(id string) (profiles.Profile, error) {
	s.mu.Lock()
	registry := s.registry
	s.mu.Unlock()
	p, err := profiles.Resolve(registry, id)
	if err != nil {
		return p, errors.New("Профиль недоступен или содержит ошибку наследования.")
	}
	return p, nil
}
func (s *Service) GetProfile(id string) (ProfileDetails, error) {
	p, err := s.resolvedProfile(id)
	if err != nil {
		return ProfileDetails{}, err
	}
	preview, err := profiles.Preview(p, nil)
	if err != nil {
		return ProfileDetails{}, errors.New("Не удалось проверить компоненты профиля.")
	}
	return ProfileDetails{p.ID, p.Name, preview.Tasks}, nil
}
func (s *Service) PreviewProfile(id string, selected []string) (ProfilePreview, error) {
	p, err := s.resolvedProfile(id)
	if err != nil {
		return ProfilePreview{}, err
	}
	// Non-nil empty selection intentionally means no components.
	if selected == nil {
		selected = []string{}
	}
	preview, err := profiles.Preview(p, selected)
	if err != nil {
		return ProfilePreview{}, errors.New("Не удалось построить план: проверьте выбор компонентов и зависимости.")
	}
	s.mu.Lock()
	var system inventory.SystemInfo
	if s.state.Report != nil && s.state.Report.System != nil {
		system = *s.state.Report.System
	}
	s.mu.Unlock()
	result := ProfilePreview{Scope: preview.Scope, Tasks: []PlannedTask{}}
	states := map[string]string{}
	for _, task := range preview.Tasks {
		status, reason := "compatible", "Требования ОС совпадают"
		ok, checkErr := inventory.Applicable(task.Requirements, system)
		if checkErr != nil {
			status, reason = "unknown", "ОС или архитектура неизвестны"
		} else if !ok {
			status, reason = "incompatible", "Требования ОС или архитектуры не совпадают"
		}
		for _, dep := range task.Dependencies {
			if states[dep] != "compatible" {
				status, reason = "blocked", "Зависимость несовместима или не проверена"
				break
			}
		}
		states[task.ID] = status
		result.Tasks = append(result.Tasks, PlannedTask{task, status, reason})
	}
	s.mu.Lock()
	s.appendLocked("info", "profiles", "Построен предварительный план. Системные изменения не выполнялись.")
	s.mu.Unlock()
	return result, nil
}
