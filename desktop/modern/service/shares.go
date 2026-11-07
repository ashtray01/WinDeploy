package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"windeploy/core/shares"
)

// ShareRequest is the explicit, user-initiated connection request. Password is
// used only for this call and is never stored in the service state.
type ShareRequest struct {
	ID              string `json:"id"`
	UNC             string `json:"unc"`
	Name            string `json:"name"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Persistent      bool   `json:"persistent"`
	Shortcut        bool   `json:"shortcut"`
	DryRun          bool   `json:"dry_run"`
	ResolveConflict bool   `json:"resolve_conflict"`
}

// ShareResultView is the safe outcome shown in the UI.
type ShareResultView struct {
	Share           string   `json:"share"`
	UNC             string   `json:"unc"`
	Connected       bool     `json:"connected"`
	CredentialSaved bool     `json:"credential_saved"`
	ShortcutPath    string   `json:"shortcut_path,omitempty"`
	Conflict        bool     `json:"conflict"`
	Message         string   `json:"message"`
	Warnings        []string `json:"warnings"`
}

// ShareConnectionView is one existing Windows connection.
type ShareConnectionView struct {
	RemoteName string `json:"remote_name"`
	LocalName  string `json:"local_name,omitempty"`
}

// ListShareConnections enumerates existing mapped/UNC connections read-only.
func (s *Service) ListShareConnections() ([]ShareConnectionView, error) {
	_, connector, _ := sharePlatform()
	connections, err := connector.List(context.Background())
	if err != nil {
		return nil, errors.New("Не удалось прочитать существующие сетевые подключения.")
	}
	views := make([]ShareConnectionView, 0, len(connections))
	for _, connection := range connections {
		views = append(views, ShareConnectionView{RemoteName: connection.RemoteName, LocalName: connection.LocalName})
	}
	return views, nil
}

// AddShare connects a UNC resource. Only an explicit user action calls it; a
// credential conflict is reported for a second, confirmed attempt.
func (s *Service) AddShare(request ShareRequest) (ShareResultView, error) {
	if !request.DryRun {
		if err := s.requireUserContext(context.Background()); err != nil {
			return ShareResultView{}, err
		}
	}
	share := shares.Share{ID: strings.ToLower(strings.TrimSpace(request.ID)), UNC: strings.TrimSpace(request.UNC), Name: strings.TrimSpace(request.Name), Username: strings.TrimSpace(request.Username), Persistent: request.Persistent, Shortcut: request.Shortcut}
	if share.ID == "" {
		share.ID = "share-" + strings.TrimLeft(strings.ReplaceAll(strings.ToLower(share.UNC), `\`, "-"), "-")
		if len(share.ID) > 64 {
			share.ID = share.ID[:64]
		}
	}
	if err := share.Validate(); err != nil {
		return ShareResultView{}, errors.New("Проверьте UNC-путь вида \\\\сервер\\ресурс и имя подключения.")
	}
	store, connector, shortcuts := sharePlatform()
	manager := &shares.Manager{Store: store, Connector: connector, Shortcuts: shortcuts}
	desktop, desktopErr := desktopDirectory()
	if desktopErr != nil && share.Shortcut && !request.DryRun {
		return ShareResultView{}, errors.New("Не удалось определить папку рабочего стола.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := manager.Add(ctx, share, shares.Options{
		Password: request.Password, DryRun: request.DryRun,
		ResolveConflict: request.ResolveConflict, ShortcutDir: desktop,
	})
	return shareOutcome(result, err, request.DryRun)
}

func shareOutcome(result shares.Result, err error, dryRun bool) (ShareResultView, error) {
	view := ShareResultView{Share: result.Share, UNC: result.UNC, Connected: result.Connected, CredentialSaved: result.CredentialSaved, ShortcutPath: result.ShortcutPath, Warnings: result.Warnings}
	if view.Warnings == nil {
		view.Warnings = []string{}
	}
	var conflict *shares.ConflictError
	if errors.As(err, &conflict) {
		view.Conflict = true
		view.Message = "На сервере уже открыто подключение с другими учётными данными (Windows Error 1219). Подтвердите переподключение: существующие сеансы к этому серверу будут закрыты."
		return view, nil
	}
	if err != nil {
		if errors.Is(err, shares.ErrShortcutRequired) {
			if result.Connected {
				view.Message = "Сетевой ресурс подключён, но обязательный ярлык не подтверждён. Операция завершена частично."
				return view, nil
			}
			return view, errors.New("Обязательный ярлык сетевого ресурса не подтверждён.")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return view, errors.New("Операция с сетевым ресурсом не завершилась по времени.")
		}
		return view, errors.New("Не удалось подключить сетевой ресурс. Проверьте путь, доступность сервера и учётные данные.")
	}
	switch {
	case dryRun:
		view.Message = "Пробный режим: система не изменялась."
	case result.Connected && result.ShortcutPath != "":
		view.Message = "Сетевой ресурс подключён, ярлык создан на рабочем столе."
	case result.Connected:
		view.Message = "Сетевой ресурс подключён."
	default:
		view.Message = "Операция завершена."
	}
	return view, nil
}
