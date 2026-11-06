# Modern GUI

Первый работающий desktop-срез: Dashboard, Inventory, просмотр профилей, DAG и журнал. Интерфейс на русском, тёмная тема, адаптация к 1366×768 и минимальному окну 1024×640. Будущие модули помечены «Скоро» и недоступны. Изменяющие систему операции через GUI отсутствуют.

## Запуск

```powershell
.\bin\WinDeploy-Windows-x64.exe
# Вместо встроенных примеров читать собственный каталог:
.\bin\WinDeploy-Windows-x64.exe --profiles .\profiles
```

Цель Modern — Windows 10/11 x64 с Microsoft Edge WebView2 Runtime. Реально проверена текущая Windows 11 x64 с WebView2 156.0.4314.58. Runtime автоматически не устанавливается. При фатальной ошибке среды выводится нативное сообщение; путь пользовательского WebView2 cache — `%LOCALAPPDATA%\WinDeploy\WebView2`. Не запускайте несколько экземпляров с одним cache одновременно. Полной проверки Windows 10, DPI/многомониторности и отсутствующего WebView2 пока нет.

## Сборка

Общий модуль сохраняет Go 1.20. Современный `desktop/modern` изолирован и требует Go 1.26.0+, проверен Go 1.26.2; Wails/CLI/npm runtime закреплены на v3.1.0. Node 24.13.0/npm 11.6.2 проверены. Все npm-версии закреплены lockfile.

Установка отдельного CLI из корня (не меняет root go.mod):

```powershell
$env:GOCACHE = Join-Path $PWD 'cache/go-build'
$env:GOMODCACHE = Join-Path $PWD 'cache/_go-mod'
$env:GOBIN = Join-Path $PWD 'cache/tools/wails-3.1'
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
Remove-Item Env:GOBIN
.\scripts\build-modern.ps1 -InstallDependencies
```

Далее достаточно `scripts/build-modern.ps1`: bindings → UI tests → TypeScript/Vite → Go tests/vet → module verification → Windows GUI EXE. На чистом checkout сначала строится UI из закоммиченных bindings, чтобы Go embed нашёл dist для генератора. EXE и dist не коммитятся. Перед пересборкой закройте своё окно WinDeploy.

Скрипт проверяет module cache с временным workspace `local/_modern-verify/go.work`, включающим root и Modern. В Go 1.26.2 обычный `go mod verify` вложенного модуля ошибочно пытался проверить отсутствующий zip локального `windeploy v0.0.0`; workspace делает оба локальных модуля main modules. Он используется только при verify; окружение восстанавливается. Root проверки остаются независимыми.

## Контракты и ограничения

- Единственный production bridge — сгенерированные Wails TypeScript bindings; HTTP/mock fallback отсутствует.
- Collector выполняется асинхронно, один scan одновременно, deadline 30 секунд. Отмена кооперативная: синхронный Windows API не обрывается принудительно. Ошибки/partial/unknown отображаются отдельно; предыдущий снимок виден до поступления нового.
- Установленные программы — видимые Uninstall entries HKLM/HKCU, не AppX/MSIX/portable. HKCU зависит от пользователя процесса. CLI и GUI сравниваются в одном пользовательском контексте. Подробнее: [Inventory](inventory.md).
- Существующие YAML встроены через `profiles/embed.go`. `--profiles` читает внешний каталог, строгий parser ограничивает размер; до 128 файлов. Неправильный YAML отражается ошибкой, ID-дубликаты отклоняют каталог. Неудачная перезагрузка сбрасывает старый registry.
- Страница показывает разрешённый состав, требования и зависимости. Параметры не экспортируются в bridge. Core BuildPlan включает зависимости выбранных компонентов. Пустой выбор даёт пустой план.
- Preview проверяет структуру/DAG и OS applicability по последнему настоящему Inventory. Compatible означает только совпадение требований ОС. Sources/installers/permissions/installed-state не проверены; операции не выполняются. Несовместимые зависимости помечаются blocked, неизвестная ОС — unknown.
- Журнал содержит настоящие события процесса, последние 200 записей в памяти. Произвольные ошибки среды не передаются в UI дословно. Перезапуск очищает журнал.

## Реальный GUI smoke

Соберите CLI независимо: `go build -o bin/WinDeploy-Inspect-Windows-x64.exe ./cmd/windeploy-inspect`.
Запустите GUI с `--smoke-port 9223`, затем из корня:

```powershell
node scripts/gui-modern-smoke.mjs
```

Обе программы должны работать под одним пользователем. В sandbox GUI требует интерактивного запуска вне файловых ограничений для пользовательского WebView2 cache; sandbox HKCU отличается от интерактивного HKCU. Playwright подключается к настоящему WebView2, отдельный browser не скачивается. Proxy variables отключаются только внутри smoke-процесса для localhost. Порт/debug tools включаются только явным флагом; при обычном запуске они выключены.

Smoke сравнивает hostname/число программ с native CLI, проверяет rescan, таблицы/поиск/сортировку, Administrator DAG, несовместимость Legacy на текущем Windows 11, журнал и два размера окна. Частные screenshots сохраняются только в игнорируемый `local/`. Тест предполагает текущую Windows 11, что явно проверяется; для другой ОС нужна соответствующая проверка требований профиля.

Root `./...` не включает Modern. Проверяйте также `scripts/test-core.ps1` с отдельным Go 1.20 toolchain. Успешный Modern EXE не доказывает совместимость общего ядра или запуск Windows 7.
