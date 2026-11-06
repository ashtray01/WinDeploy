# WinDeploy

Инструмент подготовки рабочих станций после чистой установки Windows. Проект находится на раннем этапе разработки; готового приложения пока нет.

## Целевые редакции

- Modern: Windows 10/11 x64, отдельный современный desktop UI.
- Legacy: Windows 7 SP1 x86/x64, нативный Win32 UI без WebView2.
- Общее ядро: Go 1.20, минимум зависимостей, без бизнес-логики в UI.

Совместимость с целевыми ОС требует проверок в VM. Наличие сборки или mock-тестов не подтверждает работу на Windows 7.

## Структура

- `core/` — общее функциональное ядро и интерфейсы адаптеров.
- `docs/architecture.md` — границы модулей и требования совместимости.
- `TASKS.md` — текущая задача и roadmap.
- `HANDOFF.md` — состояние и инструкция следующей сессии.
- `AGENTS.md` — правила работы с репозиторием.

## Разработка

Используйте Go 1.20 или новее. Совместимость языка и стандартной библиотеки ядра необходимо сохранять с Go 1.20; современные версии Go сами по себе не поддерживают запуск на Windows 7.

```powershell
go test ./...
go vet ./...
go build ./...
```

Если стандартный Go-кэш недоступен в sandbox, перед проверками задайте `$env:GOCACHE = Join-Path $PWD 'cache\go-build'`. Этот каталог исключён из Git.

Доступны модели задач, DAG, dry-run preview и `engine.Run`/`RunWithOptions` с Adapter (precheck/execute/verify), отменой, timeout, opt-in retries, ограниченной параллельностью и взаимным исключением ресурсов. `core/state.FileStore` сохраняет checkpoints; `engine.Resume` продолжает точный исходный план без повторения завершённых задач.

`core/inventory` реально читает Windows system/CPU/RAM/volumes/network/installed apps. `cmd/windeploy-inspect` предоставляет read-only Inventory (text/JSON) и profile preview без системных изменений. Сборка: `go build -o bin/WinDeploy-Inspect-Windows-x64.exe ./cmd/windeploy-inspect`. Возможности, команды и ограничения — в [docs/inventory.md](docs/inventory.md).

`core/profiles` поддерживает строгий YAML schema v1, наследование, клонирование, выбор компонентов и преобразование в план. Примеры и ограничения — в [docs/profiles.md](docs/profiles.md). Modern GUI показывает реальные Inventory и Profiles Preview; изменяющих систему адаптеров пока нет. Примеры не являются подтверждением установки программ.

Если стандартный module cache недоступен, задайте `$env:GOMODCACHE = Join-Path $PWD 'cache\_go-mod'`. Подчёркивание исключает скачанные пакеты из `go test ./...`. YAML-зависимость зафиксирована в go.mod/go.sum.

Не запускайте установщики и административные операции на рабочей машине. Системные адаптеры проверяются в изолированной VM.

Modern desktop: [сборка, запуск и проверки](docs/modern.md). Готовый локальный EXE — `bin/WinDeploy-Windows-x64.exe`; Wails v3.1.0 находится только в отдельном `desktop/modern` модуле.
