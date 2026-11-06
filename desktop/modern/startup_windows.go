package main

import "golang.org/x/sys/windows"

// Do not install a runtime or change the machine on startup failure.
func showStartupError() {
	text, _ := windows.UTF16PtrFromString("Не удалось запустить WinDeploy. Проверьте наличие Microsoft Edge WebView2 Runtime, права на пользовательский кэш и отсутствие другого экземпляра приложения. Системные изменения не выполнялись.")
	title, _ := windows.UTF16PtrFromString("WinDeploy — ошибка запуска")
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}
