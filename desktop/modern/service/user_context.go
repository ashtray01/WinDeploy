package service

import (
	"context"
	"fmt"
	"windeploy/core/usercontext"
)

func (s *Service) requireUserContext(ctx context.Context) error {
	check := s.userContextCheck
	if check == nil {
		check = usercontext.Native{}.Check
	}
	if err := check(ctx); err != nil {
		return fmt.Errorf("Пользовательская операция заблокирована: запустите WinDeploy под учётной записью владельца рабочего стола. Контекст пользователя не подтверждён: %w", usercontext.ErrUnconfirmed)
	}
	return nil
}
