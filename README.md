# WinDeploy

Инструмент подготовки рабочих станций после чистой установки Windows: установка программ, сеть и SMB-папки, драйверы и принтеры, политики браузеров, настройки Windows, выполнение профилей с отчётами.

## Состояние

Проект находится в статусе **preview**. Приоритет — Modern Windows 11/10 x64, включая совместимые LTSC. Backend и страницы GUI реализованы, но аудит выявил блокирующие ошибки выполнения и непроверенные сценарии. Legacy Windows 7 заморожена; её код сохранён и не блокирует Modern. Фактическое состояние: [технический аудит](docs/AUDIT.md), [ближайшие задачи](TASKS.md). Это не production-релиз.

| Компонент | Статус |
| --- | --- |
| `core/` (Go 1.20, Win7-совместимые API) | Inventory, профили, Task Engine, пакеты, сеть, SMB, драйверы, принтеры, браузеры, настройки, выполнение, отчёты |
| Modern (Wails v3 + React, Windows 10/11 x64) | Все страницы, 37 UI-тестов; Wails EXE собирается, GUI smoke на текущем хосте заблокирован WebView2 |
| Legacy (Go 1.20, нативный Walk, Windows 7 SP1 x86/x64) | Установка ПО, сеть/SMB, диагностика; реальное окно проверено на Windows 11 x64/x86, Win7 VM — нет |
| CLI | `windeploy-inspect`, `windeploy-packages`, `windeploy-run` |

## Сборка

```powershell
# Основные проверки root установленным Go (директива модуля остаётся 1.20)
go test ./... -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip'
go vet ./...
go build ./...

# Modern EXE
scripts/build-modern.ps1

# FROZEN: Legacy x64/x86 + GUI smoke только после отдельного решения
scripts/build-legacy.ps1 -GuiSmoke

# Portable/offline ZIP + SHA256SUMS
scripts/package-release.ps1 -Version 0.1.0-preview.1
```

Артефакты — `bin/` и `dist/`; release-процесс и ограничения — [docs/release.md](docs/release.md).

## Документация

- [Архитектура](docs/architecture.md)
- [Modern GUI](docs/modern.md)
- [Legacy Windows 7](docs/legacy.md)
- [Пакеты](docs/packages.md)
- [Сеть](docs/network.md)
- [Драйверы и принтеры](docs/drivers-printers.md)
- [Браузеры и настройки](docs/browsers-settings.md)
- [Выполнение и отчёты](docs/execution.md)
- [Inventory](docs/inventory.md), [профили](docs/profiles.md)
- [TASKS](TASKS.md), [HANDOFF](HANDOFF.md)

## Безопасность

Изменяющие операции требуют подтверждения и обычно прав администратора. Не запускайте установки, драйверы, принтеры, SMB и настройки на рабочей машине: используйте изолированную VM. WinDeploy не обходит лицензирование Windows и не отключает защиту.
