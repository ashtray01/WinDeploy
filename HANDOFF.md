# WinDeploy — HANDOFF

## Текущий этап и факты

2026-11-07: завершён пакет из двух задач — Modern startup recovery/diagnostics и программный контракт P0-03 reboot boundary. Preview 0.1.0-preview.1, MVP не подтверждён. Основная платформа Windows 11/10 x64; Windows 10/LTSC ещё не проверялись. Legacy Windows 7 — FROZEN. Root Go directive 1.20 сохранена, Modern Go 1.26/Wails v3.1.0. Полный аудит не повторялся; актуализация findings — docs/AUDIT.md.

Пользователь вставил содержимое .ps1 построчно в терминал: $PSScriptRoot был пуст, EXE в этой попытке вообще не запускался. Правильная команда из root: `.\scripts\start-modern-diagnostic.ps1`. Это отдельный факт от воспроизведённых агентом ошибок приложения.

## Завершённые задачи

1. Startup logger больше не переключается на io.Discard при ошибке файла: stderr остаётся доступен, успешный file log получает копию. Новые tests покрывают open failure и оба output. На хосте выявлены Access denied для startup.log и WebView2 controller 800700aa со старым UDF; прежняя гипотеза «exit 1 до main» опровергнута inittrace и console evidence. Добавлены write probe и --webview-data-dir, launcher поддерживает -WebViewDataDir/-SmokePort. Старый профиль/ACL не изменялись. Новый workspace UDF восстановил настоящий GUI runtime/DOM/bridge smoke. Smoke исправлен: Drivers по умолчанию фильтрует проблемные устройства и может не иметь строк; проверка переключает фильтр на Все и ждёт таблицу.
2. P0-03: Verify возвращает ErrRebootPending вместо nil для операций с reboot. Engine сохраняет RebootRequired, не разрешает dependencies, не повторяет такую задачу через Precheck/Execute. Runner сохраняет BootID и per-task RebootBootIDs в checkpoint, отклоняет same/unknown boot, после смены boot разрешает Verify-only. Ошибка/отмена Verify и checkpoint во время Verify сохраняют pending obligation. Вторая reboot boundary после resume требует следующую загрузку. Старые reboot sessions без boot evidence отклоняются, а ошибки до начала engine.Resume не затирают checkpoint. Rename получает read-back активного hostname. Pending не считается Success/Completed в session, CLI/report/UI.

## Коммиты

Ветка `master`, база `7bf5406`, исходное дерево чистое.

- `b1b6dac` — fix(modern): preserve startup errors and support isolated WebView data.
- `d4c4171` — fix(deploy): persist reboot barriers and resume with verification only.
- Финальный документационный commit следует за ними; hash: `git log -1 --oneline` (самоссылка не используется).

## Фактически выполненные проверки

Установленный Go 1.26.2 windows/amd64, существующие Wails/npm caches. Зависимости/toolchains не устанавливались, root directive не повышалась.

- gofmt изменённых Go файлов, git diff --check — PASS.
- Root `go test ./... -count=1 -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip'`, `go vet ./...`, `go build ./...` — PASS вне sandbox с workspace TEMP. Исключены реальные Credential Manager/registry writes; безопасные native reads/temp-file tests выполнялись.
- После последнего добавления negative boot-evidence test: `go test ./core/engine ./core/deploy ./core/inventory ./core/reporting ./cmd/windeploy-run -count=1` и соответствующий vet — PASS. Исходники runtime после финальной сборки не менялись, только tests/comment.
- Regression: pending/dependencies, Verify-only, failure/cancel, сохранение obligation во всех Verify checkpoints, file persistence/new Runner, unknown/same boot, missing historical evidence, повторная проверка, второй reboot в одной session; CLI exit codes и отсутствие повторного Execute; rename read-back; report pending summary — PASS. Boot changes моделировались DI, reboot хоста не выполнялся.
- Native `TestModernBootIdentityStableReadOnly` — PASS: repeated kernel GUID read и cancellation. Это не проверка смены GUID при реальном reboot/fast startup.
- Финальный `scripts/build-modern.ps1 -Diagnostic` — PASS: regenerated bindings (без Git changes), UI 38/38 в 16 файлах, strict tsc + Vite, Modern Go tests/vet, module verify, GUI и console EXE. Lint отсутствует, не выполнялся.
- Startup console tests — PASS; реальный console запуск показал file log Access denied и WebView2 controller HRESULT 800700aa. `GODEBUG=inittrace=1` вне sandbox показал завершение package init — main был достигнут; старая трактовка отсутствующего startup.log была неверной.
- Console EXE с `--webview-data-dir <repo>/local/webview-startup-check --smoke-port 9223`: actual desktop DOM/bridge smoke PASS. Затем финальный GUI EXE `bin/WinDeploy-Windows-x64.exe` с отдельным `<repo>/local/webview-final-gui`: smoke PASS, native hostname/app count совпали, rescan/tables/DAG/package selection/diagnostics/pages/journal/два viewport, pageErrors=0. Приложение не выполняло mutations.
- Final GUI process был жив перед smoke; запуск через Start-Process -WindowStyle Hidden, MainWindowHandle в этой проверке был 0. Не заявлять ручную проверку видимого HWND/обычного double-click по этому результату. DOM/bridge реального EXE подтверждены. Закрывались только собственные процессы проверки.

## Непроверенное и ограничения

Default UDF/видимое окно при обычном запуске владельцем, Win10/LTSC, native install/update/cancel, SMB server credentials, политики/драйверы/принтеры, реальный reboot/fast startup/sleep/VM snapshot не проверялись. Изолированной VM/сервера нет. Boot-ID change и post-reboot native verification пока DI, не VM evidence. Старые EXE/dist ZIP не перепаковывались как release. Legacy/race detector/GitHub CI не запускались; remote отсутствует. Старый Go 1.20 toolchain не устанавливался и не использовался.

## Неисправленные проблемы

- Старый WebView2 UDF давал 800700aa; точная внутренняя причина busy/permissions/profile не установлена. Fresh UDF работает; данные не сбрасывались, защиту/ACL не меняли. Пользователю запускать файл .ps1, а не вставлять его содержимое.
- P0-03 software fix реализован, VM acceptance остаётся: kernel BootIdentifier private NT API, смена при reboot/fast startup и native readiness managers требуют проверки. Недоступный/изменённый API response блокирует execution. Автозапуска после входа нет.
- Исторические reboot sessions без boot evidence нельзя автоматически считать проверенными. Нужна ручная диагностика/новый контролируемый план; не редактировать session вручную под admin.
- P0-06: исходный пользователь/SID при Run as another admin не сохраняется; P0-04: warnings/обязательные effects browser/share/printer, часть read-back и driver target version остаются gaps.
- Operation gate process-local и только package/deployment; другие standalone mutations/CLI/другие процессы не сериализованы. Sessions/cache ACL/owner, bounded parser, semantic profiles, installer process-tree cancellation, durable logs — остаются.

## Изменения архитектуры

Managers/UI/Wails не переписывались. Startup stderr fallback + явный UDF override, portable engine pending-verification contract и trusted-caller ResumeOptions.VerifyReboot. Windows API изолирован в inventory/boot_windows.go: NtQuerySystemInformation class 90, фиксированная 32-byte структура, проверка length/GUID/error; private API может измениться, fail closed. Источник layout — System Informer phnt ntexapi.h; ограничения Microsoft указаны в docs/architecture.md. Session schema v1 получила optional boot_id/reboot_boot_ids; old non-reboot sessions читаются, old reboot без evidence блокируются. Root зависимости/Go directive/Legacy не менялись. Fresh adapter после reboot делает category read-back; полнота driver/package verification всё ещё ограничена AUDIT.

## Git и артефакты

Ветка `master`, remote отсутствует. Изменения двумя отдельными runtime commits, документация третьим. EXE/cache/local logs/screenshots/temp/inventory не коммитятся. Свежие GUI/Diagnostic/Inspect EXE в bin; VCS metadata может указывать предыдущий commit + modified, это не release. После docs commit проверить git status --short и остановиться.

## Следующие 2–4 задачи

1. P0-06: fail-closed user/machine context gate либо elevated worker с исходным SID. Все user-scoped mutations не должны молча попадать в чужие HKCU/Desktop/CredMan. Unit/service coverage, two-account VM acceptance отдельно.
2. P0-04: обязательные effects и warnings в deploy (сначала UNC+shortcut и browser webapps), отрицательные tests, корректные report/UI outcomes. Не переписывать managers без причины.
3. Win11 VM acceptance для startup/reboot: обычный visible launch, controlled 7-Zip install/detection/repeat, операция с reboot → pending → реальная перезагрузка → boot GUID changed → Verify-only/dependencies/report. Проверить normal reboot/fast startup/ошибку Verify; затем отдельная Windows 10/LTSC матрица. Без VM выполнить независимые задачи 1–2, а VM оставить явно неподтверждённой.

## Точные инструкции следующему агенту

1. Git status/branch/log -20; AGENTS/HANDOFF/TASKS/README и релевантные AUDIT/architecture/execution/modern docs. Не повторять полный аудит.
2. Из root:

```powershell
New-Item -ItemType Directory -Force local/audit-temp | Out-Null
$env:GOCACHE = Join-Path $PWD 'cache/go-build'
$env:GOMODCACHE = Join-Path $PWD 'cache/_go-mod'
$env:TEMP = Join-Path $PWD 'local/audit-temp'
$env:TMP = $env:TEMP
$env:GOTMPDIR = $env:TEMP
.\scripts\build-modern.ps1 -Diagnostic
# Запускать файл, НЕ вставлять его содержимое в терминал:
.\scripts\start-modern-diagnostic.ps1 -WebViewDataDir "$PWD\local\webview-user"
# GUI вариант:
.\bin\WinDeploy-Windows-x64.exe --webview-data-dir "$PWD\local\webview-user"
# Opt-in read-only smoke, GUI запускается отдельно с --smoke-port 9223:
go build -o bin/WinDeploy-Inspect-Windows-x64.exe ./cmd/windeploy-inspect
node scripts/gui-modern-smoke.mjs
go test ./core/engine ./core/deploy ./core/inventory ./core/reporting ./cmd/windeploy-run -count=1
```

3. Native Wails/COM могут требовать разрешённый запуск вне sandbox; workspace TEMP сначала. .git/index тоже требует разрешённой записи. Не менять global identity. Startup.log может быть старым/недоступным: смотреть текущий console output. UDF override не переносит session storage и не обходит его ACL.
4. По логическому изменению проверки/commit/статус; после пакета обновить документы, clean Git и stop. Не возвращаться к Legacy/старым toolchains без отдельного решения.

## Опасные действия

Только read-only Inventory/диагностика на хосте. Реальные installers, drivers/printers, SMB credentials, policies/settings/reboot — изолированная VM. Не отключать Defender/UAC/Firewall, не менять ACL/старый WebView2 профиль для обхода ошибок, не убивать чужие процессы. 0600 не Windows ACL. Boot GUID/plan hash не аутентифицируют владельца sessions: trusted storage обязателен. Не исполнять изменённые недоверенные sessions под admin и не объявлять preview production-ready.
