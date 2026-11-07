# WinDeploy — HANDOFF

## Текущий этап и факты

2026-11-07: завершён пакет из одной задачи (по решению владельца остановлен после задачи 1) — structured task warnings / P0-04 remainder. Пользователь сообщил об успешном запуске программы. Его local/modern-diagnostic.log содержал Access denied для AppData startup.log, затем успешные WebView2 creation/штатное завершение; русские сообщения были mojibake. Preview 0.1.0-preview.1, MVP не подтверждён. Modern Windows 11/10 x64, Windows 10/LTSC VM не проверены. Legacy Windows 7 FROZEN. Root directive Go 1.20 сохранена, Modern Go 1.26/Wails v3.1.0. Аудит целиком не повторялся.

## Завершённые задачи

0. Structured task warnings (P0-04 remainder, `e6148c2`, `4a03dd7`, `2fd8799`). engine.TaskResult получил `warnings`; optional `engine.WarningReporter` (Adapter capability) отдаёт короткие переведённые уведомления, executor копирует их после Execute и Verify, `SanitizeWarnings` обрезает/дедуплицирует (≤8 × 300 runes, без CR/LF), Snapshot.Validate отклоняет некорректные warnings, поэтому они сохраняются в checkpoint/session и переживают reboot Verify-only. deploy.Adapter реализует reporter и передаёт warnings из shares (ярлык), printers (default) и browsers (Firefox startup URLs, веб-приложения) — ранее они молча терялись; browser incomplete effects по-прежнему Failed. reporting HTML/JSON показывают warnings (HTML escaped), Modern DeployTaskView.warnings отображается в таблице выполнения и в сохранённой сессии. Printer default остаётся warning (не read-back); registry/browser policy read-back не реализован.
1. Launcher явно декодирует native UTF-8, пишет UTF-8 без BOM с AutoFlush и восстанавливает PowerShell encoding/preference. По умолчанию передаёт --log-dir <repo>/local/startup, опционально -StartupLogDir; GUI тоже принимает --log-dir. Старый AppData startup.log/ACL не менялись. Пользовательский исходный лог сохранён в ignored local/modern-diagnostic-user-before.log. Real launcher smoke подтвердил корректные русские строки и отсутствие Access denied в новом логе. CDP smoke ждёт асинхронное появление страницы после подключения.
2. P0-06 software gate: core/usercontext сравнивает SID TokenUser процесса с SID владельца shell текущего desktop/session (GetShellWindow → PID → token). Unknown/mismatch/ошибка чтения fail closed, DLL Find failure возвращает отказ вместо panic. Никакого имени из env/argv, impersonation или запроса привилегий. Gate применён в whole-plan Runner start/resume, фазах deploy.Adapter, user-scoped Modern actions и CLI install. Пакеты консервативно user-scoped, так как каталог не задаёт scope; ContextRunner повторяет check прямо перед installer EXE после downloads/retries. Machine-only настройки и read-only/preview не требуют совпадения SID. Same-user UAC не блокируется по одному признаку elevation.
3. P0-04 selected effects: shares.ShortcutInspector подтверждает сохранённый .lnk target/arguments через COM без Resolve/запуска цели. UNC без правильного required shortcut не AlreadyInstalled; missing/wrong link возвращает Pending; repair существующего UNC без явно заданных credentials/conflict не переподключает и не трогает Credential Manager. Required creation/read-back failure возвращает ErrShortcutRequired с partial Connected state. Browser webapps missing dependency/browser проверяются до policy writes; Create failure не Success. Production browser shortcut тоже проходит .lnk read-back. Deploy browser warnings fail closed как incomplete outcome. Service сообщает о partial share, UI очищает старый Success перед новым запросом. Warnings теперь сохраняются (см. задачу 0); printer/registry read-back остаётся открытым.

## Коммиты

Ветка master, база 6724076, исходное дерево было чистое.

- a82f11e — fix(modern): write readable diagnostics to an explicit log directory.
- c9e1839 — fix(context): gate user mutations on the interactive shell SID.
- bab8e65 — fix(deploy): require verified share and web-app shortcuts.
- e6148c2 — feat(engine): persist task effect warnings in results and reports.
- 4a03dd7 — fix(deploy): report unconfirmed share, printer and browser effects.
- 2fd8799 — feat(modern): show task effect warnings in execution view.
- Финальный docs commit следует за ними; hash — git log -1 --oneline, самоссылки нет.

## Фактически выполненные проверки

Эта итерация (задача 0, warnings). Установленный Go 1.26.2 windows/amd64 и существующие Wails/npm caches; зависимости/toolchains не устанавливались.

- gofmt изменённых Go файлов (`core/engine`, `core/deploy`, `core/reporting`, `desktop/modern/service`) — PASS.
- go vet ./core/engine ./core/deploy ./core/reporting ./core/shares ./core/printers ./core/browsers ./cmd/windeploy-run ./cmd/windeploy-packages — PASS.
- go test этих пакетов -count=1 — PASS, включая новые TestWarningsAreCollectedAndSanitized/BoundedAndPersisted/KeptOnFailureAndReboot/SnapshotRejectsInvalidWarnings, TestReportIncludesTaskWarnings, TestShareWarningIsPersistedWithOutcome, TestPrinterDefaultWarningIsPersisted, TestBrowserWarningIsPersistedOnIncompleteEffects.
- Modern: go vet/test ./service — PASS (включая TestGetDeploymentSessionShowsTaskWarnings); scripts/build-modern.ps1 без -Diagnostic — PASS: bindings обновились (models.ts `warnings`), UI 41/41 в 16 файлах (новый тест отображения warnings), strict tsc/Vite, Modern Go tests/vet, module verify, GUI EXE. Lint отсутствует, не выполнялся.
- go build ./... — PASS. Полный `go test ./...` — **нестабилен в этой среде**: несколько прогонов дали FAIL в `core/state`/`core/deploy`/`cmd/windeploy-run` с "could not save deployment checkpoint"/rename error в workspace TEMP (OneDrive); изолированные прогоны тех же пакетов PASS (6/6 и 6/6). Причина не локализована; это ограничение проверки, а не подтверждённый дефект warnings. По решению владельца пакет остановлен после задачи 0.
- Реальный GUI/DOM smoke этой итерации не выполнялся; сборка EXE не подтверждает runtime.

Предыдущая итерация (diagnostic log, P0-06, P0-04 shortcuts):

- Root go test ./... -count=1 -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip', go vet ./..., go build ./... — PASS вне sandbox с workspace TEMP. Исключены реальные Credential Manager/registry writes; read-only native/temp-file tests выполнялись.
- Targeted core/usercontext/deploy/packages/CLI и Modern service test/vet/build — PASS. Negative policy: same/different/unknown SID, reader error/cancel; denied whole-plan start/resume adapter calls=0; recheck перед Execute/installer; user-scoped service endpoints reject до native calls; CLI rejected install не скачивает artifact.
- Native TestNativeIdentityReadOnly вне sandbox: process/shell SIDs совпали, PASS. Sandbox user32 load сначала дал DLL init panic; Native.Read теперь проверяет доступность процедур и возвращает fail-closed отказ. Финальный root native test PASS. Это не two-account VM evidence.
- Native TestNativeShortcutReadbackTargetAndArguments — PASS: временный локальный .lnk, target/arguments equality, wrong target/arguments, missing/corrupt file. Цели не запускались, Desktop/профили не менялись. Existing native temporary-link/list tests тоже PASS вне sandbox. Sandbox ранние COM/KnownFolder tests падали DLL init; не считать это production result.
- Core negative required effect tests — PASS: nil-success creator без read-back не accepted; Connected partial не теряется; existing UNC repair без reconnect; deploy repeat/Verify rejects wrong target; incomplete browser effects не разрешают dependency; required missing browser не пишет policy; failed webapp не Success.
- Финальный scripts/build-modern.ps1 -Diagnostic — PASS: generated bindings без Git changes, UI 40/40 в 16 файлах, strict tsc/Vite, Modern Go tests/vet, module verify, GUI и console EXE. Lint отсутствует, не выполнялся. UI regression покрывает partial share и отсутствие старого success banner после новой ошибки.
- Реальный launcher/UTF-8 DOM/bridge smoke — PASS; Russian startup=True, access denied=False в новом diagnostic log. Первый smoke обнаружил race: CDP уже доступен, page ещё нет; smoke теперь ждёт page, повтор PASS.
- Финальный GUI EXE с --webview-data-dir local/webview-context-final --log-dir local/startup-final --smoke-port 9223 — actual DOM/bridge smoke PASS, native hostname/app count matched, rescan/tables/DAG/catalog/diagnostics/pages/journal/два viewport, pageErrors=0. Повторный запуск с аргументами --key=value также PASS. Startup file читается с русским текстом, Access denied не наблюдался. Агент запускал Hidden; пользователь отдельно сообщил об успешном запуске. Закрывались только собственные тестовые процессы.

## Непроверенное и ограничения

Реальные установки/обновления/отмена пакетов, SMB server/credentials, user vs another-admin VM, политики, драйверы/принтеры, reboot/fast startup, Win10/LTSC не проверялись. Изолированной VM/сервера нет. Gate не elevated worker и не доказывает owner/trust session files; библиотечные core managers/Legacy direct callers вне новых Modern/CLI entrypoints не получили универсальную impersonation boundary. Race detector/GitHub CI не запускались, remote отсутствует. Root Go 1.20 toolchain не устанавливался, directive сохранена. Legacy не запускалась.

## Неисправленные проблемы

- P0-06 two-account VM acceptance и сценарии RDP/shell replacement/headless: отсутствующий shell намеренно блокирует user mutations. Нет worker/исходного SID в IPC и криптографической аутентификации session owner. Gate process/shell не проверяет владельца уже сохранённого файла сессии.
- P0-04 remainder: printer default/target read-back (default теперь сохраняется как warning, не подтверждается), Windows/browser registry policies read-back. Warnings теперь сохраняются в result/session/report/UI; остаются только read-back gaps. Исторические Success sessions не перепроверяются автоматически.
- Полный `go test ./...` в workspace на OneDrive нестабилен (см. проверки): изолированные прогоны PASS. Нужна диагностика (реальный TEMP вне синхронизации, антивирус) до заявления о зелёном полном прогоне.
- P0-03 software boundary была реализована раньше; настоящая смена kernel BootID, reboot readiness и fast startup по-прежнему требуют VM. Old reboot без evidence fail closed, autostart отсутствует.
- Sessions/cache ACL/owner, bounded deploy JSON, process-level writer lock; semantic profiles/check_ref; update target version; installer process tree; persistent safe diagnostics остаются открытыми.
- Old default WebView2 UDF busy причина не локализована; explicit UDF работает, старый профиль/ACL не меняли. Default GUI log всё ещё AppData, только diagnostic launcher явно выбирает local/startup.

## Изменения архитектуры

Небольшой независимый core/usercontext (policy + Windows token adapter) и packages.ContextRunner, DI guards в Runner/Adapter/Service/CLI. Платформенный код вне UI, без новых dependencies и повышения root Go. Legacy исходники/tests не менялись. Scope классифицируется до начала всего deploy плана; user context перепроверяется на adapter phases и перед installer. Совпадение SID не считается достаточной проверкой session-file ACL/owner.

shares.ShortcutCreator сохранён, дополнительный ShortcutInspector реализован production WindowsShortcut; при required shortcut отсутствие inspector fail closed. COM Create/Read привязаны к одному OS thread, S_OK/S_FALSE корректно балансируют CoUninitialize, read-back без Resolve. Browser adapter использует тот же reader. Result partial state сохраняется.

Task warnings: engine.TaskResult.Warnings (bounded, sanitized, no raw errors) — единственный канал «эффект применён, но не подтверждён». Adapter опционально реализует engine.WarningReporter; executor собирает warnings после Execute и Verify, Snapshot.Validate их проверяет, поэтому они попадают в checkpoint, session, отчёт (JSON + escaped HTML) и Modern DeployTaskView.warnings. Новые категории должны отдавать такие notices через adapter.noteWarning, а не через свободный текст ошибки. --log-dir меняет только startup log, не session/cache trust и не WebView2 profile.

## Git и артефакты

Ветка master, remote нет. Три runtime commits этой итерации и завершающий docs commit. Runtime artifacts/logs/screenshots/inventory/cache/EXE не коммитились. Во время работы появился пользовательский untracked temp/windeploy-icon.png; файл не изменялся/не добавлялся/не удалялся. Поэтому итоговый status может показывать ?? temp/ при чистых tracked файлах. Не прятать его .gitignore ради clean status. Финальные EXE собраны до docs commit, metadata может содержать предыдущий revision+modified; это не release. После docs commit выполнить git status --short и остановиться.

## Следующие 2–4 задачи

1. P1 security: bounded deploy.Store.Load (limit до allocation, trailing JSON/ID match), затем trusted Windows storage ACL/owner перед elevated resume; parser/temp-file tests и отдельная VM ACL acceptance. Не считать mode 0600 достаточным. (Следующая по плану; в этой итерации не начата по решению владельца.)
2. P0-04 remainder: printer default/target read-back и browser/Windows policy read-back с отрицательными tests; warnings уже сохраняются — нужен именно read-back, не расширение printer features.
3. Диагностика нестабильного полного `go test ./...` (workspace TEMP на OneDrive): воспроизвести, проверить реальный TEMP вне синхронизации/Defender, не маскировать flake retry-логикой.
4. Win11 VM acceptance: два пользователя (обычный user + another admin), подтверждение отказа user mutations до effects; same-user UAC; SMB+required shortcut/credentials на тестовом сервере; одобренный 7-Zip install/detection/repeat/update/failure/cancel и reboot → Verify-only. Потом отдельная Windows 10/LTSC матрица. Без VM явно передавать acceptance открытой.

## Точные инструкции следующему агенту

1. git status, branch, log -20; AGENTS/HANDOFF/TASKS/README и релевантные architecture/execution/network/browser/AUDIT docs. Полный аудит не повторять. temp/windeploy-icon.png — пользовательский файл.
2. Из root:

```powershell
New-Item -ItemType Directory -Force local/audit-temp | Out-Null
$env:GOCACHE = Join-Path $PWD 'cache/go-build'
$env:GOMODCACHE = Join-Path $PWD 'cache/_go-mod'
$env:TEMP = Join-Path $PWD 'local/audit-temp'
$env:TMP = $env:TEMP
$env:GOTMPDIR = $env:TEMP
.\scripts\build-modern.ps1 -Diagnostic
# Запускать файл, а не вставлять содержимое:
.\scripts\start-modern-diagnostic.ps1 -WebViewDataDir "$PWD\local\webview-user"
# Launcher startup log: local/startup/startup.log; UTF-8 console copy: local/modern-diagnostic.log.
# GUI напрямую:
.\bin\WinDeploy-Windows-x64.exe --webview-data-dir "$PWD\local\webview-user" --log-dir "$PWD\local\startup"
# Smoke: GUI отдельно с --smoke-port 9223, только read-only:
go build -o bin/WinDeploy-Inspect-Windows-x64.exe ./cmd/windeploy-inspect
node scripts/gui-modern-smoke.mjs
go test ./core/usercontext ./core/deploy ./core/packages ./cmd/windeploy-run ./cmd/windeploy-packages -count=1
go test ./core/shares ./core/browsers -count=1 -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip'
go test ./core/engine ./core/deploy ./core/reporting -count=1   # warnings/read-back regression
# Полный прогон: go test ./... -count=1 -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip'
# В этой среде он нестабилен; при FAIL сначала повторить пакет изолированно и записать результат честно.
```

3. Native Wails/COM tests/build могут требовать разрешённого запуска вне sandbox; сначала workspace TEMP. Git index тоже может быть read-only в sandbox. Не менять global Git identity. DI allow guards существуют только для fake tests; production nil guard вызывает Native, не bypass.
4. По каждой логической задаче diff/gofmt/test/vet/build/commit и статус; после пакета документы и stop. Следующие крупные этапы автоматически не начинать, Legacy не трогать.

## Опасные действия

Read-only Inventory/диагностика допустимы на хосте. Установки, реальные credentials/SMB, драйверы, принтеры, policies/settings/reboot — изолированная VM. Не отключать Defender/UAC/Firewall и не менять чужие ACL/профили, не убивать процессы пользователя. SID gate не impersonation и не authorization для недоверенной session; trusted ACL/owner остаются обязательными. Не публиковать preview как production-ready; не коммитить logs/EXE/credentials/inventory.
