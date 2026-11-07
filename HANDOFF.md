# WinDeploy — HANDOFF

## Текущий этап и факты

2026-11-07: завершён пакет Modern startup diagnostics + P0-02 + P0-05. Modern Windows 11/10 x64 остаётся preview 0.1.0-preview.1, MVP не подтверждён. Windows 7/Legacy — FROZEN. Root go.mod остаётся 1.20; Modern — отдельный Go 1.26/Wails v3.1.0 модуль. Полный исходный аудит — docs/AUDIT.md; его не повторяли. Пользователь сообщил, что EXE из bin не запускаются. Настоящий GUI запуск в этой итерации также не удался.

## Завершённые задачи

1. Modern build использует `-tags production`. Добавлены опциональный console EXE (`-Diagnostic`) и `scripts/start-modern-diagnostic.ps1`: stderr ранних dependency init failures сохраняется в local/modern-diagnostic.log, ненулевой exit виден в терминале. README отличает Modern GUI от CLI и старых bin artifacts. Это диагностика/упаковка, а не устранённый отказ окна.
2. P0-02: единая граница deploy.Runner на Start/Resume получает fresh System через DI или Native.System, проверяет ВЕСЬ план до adapter calls, затем оборачивает все фазы GuardAdapter. Unknown/read error/mismatch/cancel fail closed. Session.System используется как metadata, не authorization; успешный resume сохраняет свежий target. Modern preparation тоже читает provider, не cached Inventory. Отклонённый resume не получает Completed=true в live view.
3. P0-05: process-local operationGate общий для StartPackageInstall/StartDeployment/ResumeDeployment; резервируется до подготовки, освобождается на ошибке или после finish. Start/resume повторно проверяют closed/context/running перед wg.Add. Shutdown закрывает admission gate и отменяет контексты. Barrier tests проверяют подготовку, активную установку, два resume и shutdown.

## Коммиты

Ветка `master`, исходное дерево чистое, база `1079b5a`.

- `90d9905` — fix(modern): build production assets and expose early startup failures.
- `d81df3e` — fix(deploy): enforce fresh target requirements before start and resume.
- `2d3562c` — fix(modern): serialize package and deployment writer lifecycle.
- Завершающий docs commit следует за ними; точный hash: `git log -1 --oneline` (самоссылка не используется).

## Фактически выполненные проверки

Установленные Go 1.26.2 windows/amd64 и существующие npm/Wails caches; зависимости/toolchains не устанавливались.

- gofmt изменённых Go файлов и `git diff --check` — PASS.
- `go test ./core/deploy ./core/inventory ./cmd/windeploy-run -count=1`, соответствующие vet/build — PASS. Новые negative tests требуют adapter calls=0 для start/resume mismatch/unknown/read-error/cancel; rejected resume не меняет checkpoint. Есть CLI unknown target и service cached-vs-fresh target tests. DI tests не являются native mutations E2E.
- Root `go test ./... -count=1 -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip'`, `go vet ./...`, `go build ./...` — PASS вне sandbox с workspace TEMP. Две реальные системные записывающие проверки исключены. Остальные native read-only/temp-file tests выполнялись.
- Modern `go test ./... -count=1`, `go vet ./...`, production go build — PASS вне sandbox с workspace TEMP.
- Конкурентные service tests `TestDeploymentPreparationExcludesOtherWriters|TestPackageExecutionExcludesDeployment|TestShutdownDuringDeploymentPreparation` — PASS, включая `-count=20`.
- Финальный `scripts/build-modern.ps1 -Diagnostic` — PASS: regenerated bindings (Git без изменений), UI 37/37 в 16 файлах, strict tsc, Vite, Modern Go test/vet, module verify, GUI и diagnostic EXE. Lint отсутствует, не запускался.
- В sandbox полный build останавливается на init user32 у Wails CLI. Console production EXE в sandbox показывает `DLL initialization routine failed` в go-ole.IIDFromString → Wails internal cfd.init до main. Диагностический launcher сохранил stack trace и вывел exit 2. Эта причина подтверждена только для sandbox запуска.
- Вне sandbox существующий и свежий GUI EXE завершались exit 1, новых строк в startup.log не было. `node scripts/gui-modern-smoke.mjs` — FAIL `Desktop WebView2 unavailable`; CDP/DOM/bridge не достигнуты. Финальный diagnostic EXE через launcher вне sandbox — exit 1 без stderr, startup.log не обновлён. Причина этого внешнего exit 1 не локализована; нельзя выдавать sandbox DLL panic за её доказательство. Своих оставшихся GUI процессов нет.

## Непроверенное и ограничения

GUI HWND/DOM/native comparison, Windows 10/LTSC, установки/обновление/отмена ПО в VM, SMB server credentials, политики, драйверы, принтеры и reboot не проверены. Изолированной VM/тестового сервера нет. Legacy не запускалась. Race detector не запускался (gcc/clang в PATH не найдены); конкурентные barrier tests не заменяют race detector. CI/remote/release не запускались. Старые dist ZIP не обновлены и не содержат текущие fixes.

## Неисправленные проблемы

- Приоритет: Modern EXE выходит до startup.log; исправление production tag не восстановило окно. Нужен интерактивный запуск владельцем с console output, затем read-only desktop smoke. Не лечить слепой переустановкой WebView2 или отключением защиты.
- P0-03: reboot Verify пропускается, непроверенный Success разрешает dependencies и сохраняется Resume. Модель deferred verification/boot identity не реализована.
- P0-04: warnings/обязательные эффекты компонентов теряются, browser/settings read-back отсутствует.
- P0-06: HKCU/Desktop/CredMan под другим admin могут относиться не к исходному пользователю; worker/context gate отсутствует.
- Gate P0-05 покрывает только package/deployment в одном Service. Другие standalone mutations и CLI/другие процессы не сериализованы; session ACL/owner/multiprocess lock остаются открытыми.
- Semantic parameters/check_ref, target version detection, bounded session parsing, process-tree cancellation, durable logs, printer TCP/IP port — см. AUDIT/TASKS.

## Изменения архитектуры

Сохранены существующие managers, bridge, engine и UI. Добавлены DI ReadSystem и fail-closed plan guard в Runner, process-local admission lease в Modern Service, production/console build modes. Root Go directive/dependencies и Legacy не менялись. Контракт reboot не менялся. Обновлены docs/architecture.md и docs/modern.md, чтобы отличать реализованные gates от оставшихся планов.

## Git и артефакты

Ветка `master`, remote отсутствует. Каждый логический change отдельным commit, финальная документация отдельно. EXE, caches, local logs/temp/screenshots, персональный inventory не добавлены. Свежие `bin/WinDeploy-Windows-x64.exe` и `bin/WinDeploy-Diagnostic-x64.exe` собраны до завершающих commits; embedded VCS revision может указывать предыдущий commit + modified. Это не release. После docs commit выполнить `git status --short` и остановиться.

## Следующие 2–4 задачи

1. Восстановить настоящий Modern startup: интерактивный console EXE на Windows 11, установить причину exit 1 до main, минимальное исправление только по доказательствам; затем HWND/CDP/DOM/native Inventory/bridge read-only smoke. CLI запускать из терминала. Отдельно проверить Win10/LTSC позднее.
2. P0-03: честная reboot boundary — согласованный engine/deploy/persistence/reporting/UI контракт; dependencies ждут verify, resume не переустанавливает завершённое, отрицательные/restart tests. VM gate явно отдельно.
3. P0-06: fail-closed user/machine context gate или elevated worker с исходным SID; two-account VM verification до реальных пользовательских настроек.

P0-04 следующий после этого пакета. Новые features и Legacy не начинать; не повторять полный аудит.

## Точные инструкции следующему агенту

1. `git status`, `git branch --show-current`, `git log -20 --oneline`; AGENTS/HANDOFF/TASKS/README и релевантные AUDIT/architecture/modern/execution docs.
2. Для существующих caches (из root):

```powershell
New-Item -ItemType Directory -Force local/audit-temp | Out-Null
$env:GOCACHE = Join-Path $PWD 'cache/go-build'
$env:GOMODCACHE = Join-Path $PWD 'cache/_go-mod'
$env:TEMP = Join-Path $PWD 'local/audit-temp'
$env:TMP = $env:TEMP
$env:GOTMPDIR = $env:TEMP
scripts/build-modern.ps1 -Diagnostic
scripts/start-modern-diagnostic.ps1
# После успешного интерактивного GUI запуска с --smoke-port 9223:
go build -o bin/WinDeploy-Inspect-Windows-x64.exe ./cmd/windeploy-inspect
node scripts/gui-modern-smoke.mjs
# Backend:
go test ./core/deploy ./core/inventory ./cmd/windeploy-run -count=1
# Modern отдельно:
Push-Location desktop/modern
go test ./... -count=1
go vet ./...
Pop-Location
```

3. Startup log `%LOCALAPPDATA%\WinDeploy\logs\startup.log` может быть СТАРЫМ. Сравнивать timestamp до/после; отсутствие новой записи означает, что main мог не выполниться. Console stderr помогает только если Windows/runtime его отдаёт. Не путать успешный build с успешным GUI.
4. Native DLL/Wails build/test требуют разрешённого запуска вне sandbox. Сначала workspace TEMP. Git index также read-only в sandbox, commit требует разрешённой записи .git. Не менять Git identity.
5. По логическому изменению diff/gofmt/test/vet/build/commit и статус; после пакета docs и clean Git. Не устанавливать старые toolchains.

## Опасные действия

Не запускать реальные installers/драйверы/принтеры/SMB credentials/политики/настройки на хосте; только изолированная VM. Read-only Inventory/диагностика допустимы. Не отключать Defender/UAC/Firewall, не запускать неизвестные EXE, не переустанавливать runtime без основания. Не считать 0600 Windows ACL-защитой, не исполнять недоверенную session под admin. Operation gate не является межпроцессной блокировкой. Не публиковать preview как production-ready.
