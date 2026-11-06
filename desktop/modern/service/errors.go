package service

// RuntimeErrorHandler records actual framework errors without exporting raw
// diagnostics (which may contain paths or user data) through the bridge.
func RuntimeErrorHandler(s *Service) func(error) {
	return func(err error) {
		if err == nil {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.appendLocked("error", "application", "Ошибка среды приложения. Проверьте WebView2 Runtime и доступность пользовательского кэша.")
	}
}
