# WinDeploy technical audit — 2026-11-07

## Public beta preparation — 2026-11-08

Полный аудит не повторялся. База — `master` `9010d95`; проверялись только
release-blockers. Сессии/ACL, SID gate и обязательные read-back эффекты уже были
исправлены в `ddc806f`/`9010d95`; текущая итерация добавила единый `VERSION`,
Modern-only CI/release, SHA-256/portable packaging, публичные документы и явный
preview-before-execute.

Фактически: root Go 1.20.14 test/vet/build amd64+386 PASS; Modern Go 1.26.2
tests/vet PASS; frontend 46/46, strict TypeScript/Vite PASS; production EXE
собран, version resource `0.1.0-beta.1`; actual GUI EXE smoke PASS с native
Inventory, двумя viewport и pageErrors=0. Реальные установка пакета, SMB-сервер,
reboot, драйверы/принтеры и VM не выполнялись. Windows 10/LTSC не заявляются;
Legacy FROZEN.

Актуальная матрица и блокеры — [BETA_READINESS.md](BETA_READINESS.md); известные
ограничения — [KNOWN_ISSUES.md](KNOWN_ISSUES.md). Следующие действия — TASKS.md.

## Итерация warnings (P0-04 remainder, частично)

База `bab8e65`; коммиты `e6148c2`, `4a03dd7`, `2fd8799`. Полный аудит не повторялся.

- `engine.TaskResult` получил bounded `warnings`; optional `engine.WarningReporter`, `SanitizeWarnings`, проверка в `Snapshot.Validate`. Warnings копируются executor-ом после Execute и Verify, поэтому попадают в checkpoint, session, JSON/escaped HTML отчёт и Modern `DeployTaskView.warnings`.
- `deploy.Adapter` передаёт warnings из shares/printers/browsers вместо отбрасывания; browser incomplete effects по-прежнему Failed. Printer default остаётся warning, registry read-back не реализован.
- Проверки: targeted vet/test PASS, Modern service tests PASS, `scripts/build-modern.ps1` PASS (UI 41/41, strict tsc/Vite, module verify, GUI EXE). Полный `go test ./...` в workspace на OneDrive **нестабилен** (FAIL сохранения checkpoint/rename в `core/state`/`core/deploy`); изолированные прогоны PASS. Причина не установлена; не считать полный прогон зелёным.
- Реальный install/SMB/printer/reboot/VM не выполнялись; GUI/DOM smoke этой итерации не запускался.

## Последующая итерация: startup и reboot boundary

Текущая база результатов — `b1b6dac` и `d4c4171`; разделы ниже описывают исторический аудит/предыдущую итерацию и не отменяют эти уточнения. Полный аудит повторно не выполнялся.

- Startup: прежнее предположение «нет startup.log → не достигнут main» оказалось неверным. Вне sandbox Go inittrace проходит все пакеты, а исправленный stderr logger показал startup.log Access denied и WebView2 controller 800700aa. Раньше io.Discard скрывал эти ошибки. Добавлены stderr fallback/tests, UDF write probe и explicit --webview-data-dir без изменения старого профиля/ACL. Пользовательская попытка вставить текст .ps1 в терминал отдельно не доходила до EXE из-за пустого $PSScriptRoot; запускать нужно файл.
- Console и финальный GUI EXE с новым workspace UDF: actual DOM/bridge smoke PASS, native hostname/app count совпали, rescan/tables/DAG/catalog/diagnostics/pages/journal/два viewport, pageErrors=0. Smoke выбирает Все устройства: default problems filter законно пуст. Проверка запускалась Hidden, MainWindowHandle=0; visible/default-UDF double-click владельцем и Windows 10/LTSC не подтверждены. Точная причина busy старого UDF остаётся не локализована.
- P0-03 software contract исправлен: ErrRebootPending сохраняется как RebootRequired, не Success/Completed; dependencies blocked; file checkpoint сохраняет boot_id/per-task origin; same/unknown boot fail closed. После смены boot engine вызывает Verify-only без Precheck/Execute; failure/cancel/checkpoint сохраняют obligation. Второй reboot после resume требует новую загрузку. Historical reboot sessions без evidence не восстанавливаются автоматически. Rename проверяет active hostname. CLI/report/UI отражают pending.
- Root safe test/vet/build PASS, финальная Modern build PASS (bindings, UI 38/38, strict TS/Vite, Go test/vet, module verify, GUI/Diagnostic). Native boot GUID repeated read/cancel PASS на текущем Win11. Regression tests покрывают simulated reboot/restart/persistence/failed verify/cancel/second reboot, CLI/service/report/UI. **Реальный reboot/install/fast startup не выполнялись: P0-03 VM acceptance остаётся открытой.**
- Boot query изолирован в Windows adapter, использует private NT class 90 с validation/fail-closed. Layout — [первичный phnt header](https://github.com/winsiderss/systeminformer/blob/master/phnt/include/ntexapi.h); [Microsoft описывает изменяемость NT query](https://learn.microsoft.com/en-us/windows/win32/api/winternl/nf-winternl-ntquerysysteminformation). Это ограничение архитектуры, не обещание поддержки всех builds. Boot evidence не заменяет session ACL/owner authentication.

Следующий пакет: P0-06 user/machine context, P0-04 required effects/warnings, Win11 VM startup/install/reboot acceptance; подробности — TASKS/HANDOFF. Legacy не запускалась.

## Controlled development update — после базы аудита

Этот раздел уточняет исторические findings ниже; полный аудит не повторялся. Текущие команды/ограничения — [HANDOFF](../HANDOFF.md).

- `90d9905`: Modern production build tag и optional console diagnostic EXE/launcher. Полная финальная Modern сборка PASS (bindings, UI 37/37, strict TS/Vite, Go test/vet, module verify, EXE). **GUI startup остаётся FAIL**: вне sandbox GUI/console выходят 1 без новой записи startup.log; причина не установлена. Sandbox console отдельно показывает COM DLL initialization panic в go-ole/Wails cfd.init до main. GUI smoke — `Desktop WebView2 unavailable`, DOM/bridge не достигнуты. Эти два наблюдения не доказывают общую причину.
- P0-02 исправлен `d81df3e`: Runner Start/Resume читает fresh target, preflight всего плана до adapter phases, GuardAdapter на все фазы. Cached Inventory и Session.System не разрешают mutation. Core mismatch/unknown/read-error/cancel tests: adapter calls=0; CLI/service negative tests PASS. Успешный resume обновляет report/session System. Отклонённый resume не получает Completed в live view.
- P0-05 исправлен для package/deployment одного Modern Service, `2d3562c`: operation lease охватывает preparation→finish; повторные closed/context/running checks, shutdown admission close. Barrier service tests и 20 повторов PASS. **Не покрыты другие standalone mutations, CLI/другие процессы и межпроцессный session lock.**
- Итоговые root test/vet/build PASS вне sandbox с workspace TEMP; два записывающих Credential Manager/registry tests исключены. Legacy/VM/reboot/mutations не выполнялись. Race detector не запускался, compiler для него в PATH не найден. Ни тесты DI, ни сборка не подтверждают GUI runtime или Windows 10/LTSC.

Остаются P0 startup, P0-03/04/06 и остальные gaps. Следующий пакет: настоящий startup/read-only desktop smoke, reboot boundary, user/machine context. Historical evidence matrix и описания исходных дефектов ниже сохраняются как база аудита.

## Project Status

База аудита: `master`, `b990439`; рабочее дерево в начале чистое, remote отсутствует. Изучены последние 20 коммитов, три Go-модуля, production-пути core/CLI/Modern, тесты, конфигурации и workflows. Это выборочный технический анализ всех подсистем, а не доказательство отсутствия ошибок во всех строках. Исправление этой сессии: `972e44e`.

Версия упаковки — `0.1.0-preview.1`, frontend — `0.1.0`; единого источника версии нет (HTTP User-Agent в `core/packages/fetch.go` содержит 0.2). Рабочий MVP для массового применения **не подтверждён**. Основная цель теперь Windows 11 x64 и Windows 10 x64, включая совместимые LTSC. Конкретные LTSC builds/WebView2 нужно проверить; широкий OS token не доказывает совместимость.

Root `go.mod`: Go 1.20, yaml v3.0.4, x/sys v0.6.0. `desktop/modern`: Go 1.26.0, Wails v3.1.0, React/TypeScript; `replace windeploy => ../..`. `legacy`: отдельный Go 1.20/Walk, **FROZEN**, исходники и тесты сохранены. Версии зависимостей зафиксированы, актуальность/CVE не подтверждалась онлайн; автоматического обновления зависимостей не было.

На текущей Windows x64 реально проходят безопасные native reads и локальные тесты; root и Modern собираются. Установка стороннего ПО, драйверов, принтеров, SMB с реальным сервером, политики и изменения Windows не проверялись в этой сессии. Исторические результаты HANDOFF не считаются новым запуском. Windows 10/LTSC VM и Win7 VM не доступны; Win7 не блокирует Modern.

Для локальной диагностики пригодны Inventory и read-only CLI. Для изменяющего развёртывания остаются P0 ниже. Наличие страниц GUI и fake-тестов не означает готовность операций.

## Component Matrix

«GUI: да» означает найденный маршрут page → bridge → service → core, а не подтверждённый desktop E2E. READY здесь не присваивается подсистемам, проверенным только fake-тестами.

| Компонент | Backend | GUI | Тесты | Реальная проверка | Статус |
|---|---|---|---|---|---|
| Task Engine | DAG, precheck/execute/verify, retry, cancel, resources | Выполнение | executor/parallel/retry/checkpoint/resume | read-only deploy service/CLI tests | PARTIAL: интеграционные контракты нарушены адаптером |
| Persistent State | state.FileStore + deploy.Store, разные форматы | Сессии/resume | file/session/deploy tests | локальные файлы и отчёты | PARTIAL: ACL, reboot, конкурентные writers |
| Profile Manager | strict YAML, inheritance, selection closure | Профили + Выполнение | profiles/*, service/profiles_test | unit и service | PARTIAL: нет семантической проверки всех параметров |
| Inventory | Windows API/registry, partial errors | Обзор, таблицы | native/collector/applicability | реальные read-only Windows tests | READY для проверенного текущего хоста; VM-матрица открыта |
| Package Manager | detection, plan, fetch, trust, install | Установка ПО, Выполнение | packages/* + CLI/service/UI | native trust/probes; установок нет | PARTIAL: P0-01 исправлен, остались gaps |
| Network | DNS/TCP/HTTP/ICMP/DHCP/gateway/445 | Диагностика | native + fake + UI | native diagnostics на текущем хосте | PARTIAL: profile parsing, GUI cancel |
| SMB | WNet, CredMan, IShellLink | Сетевые папки | fake + native list/link | list и временный .lnk; без SMB server | PARTIAL |
| Drivers | SetupAPI/CM, INF verification, pnputil | Драйверы | fake + native enumeration | read-only devices | UNVERIFIED для установки |
| Printers | EnumPrinters, PrintUI, default/GDI | Принтеры | fake + native enumeration | read-only queues | BROKEN для создания отсутствующего TCP/IP port |
| Browsers | Chromium/Firefox registry policies, webapp links | Браузеры | fake-store + UI | запись политик исключена | PARTIAL: warnings теряются в deploy |
| Windows Settings | rename/tzutil/Explorer/powercfg; update/licensing reads | Windows | fake + native reads + UI | диагностика; без изменений | PARTIAL: нет read-back/rollback |
| Logging | bounded in-memory journals, startup file | Журнал + журналы страниц | service/UI | service tests | PARTIAL: нет общего durable operation log |
| Reporting | escaped HTML/JSON, exclusive files | Пути отчётов выполнения | reporting/deploy/CLI | реальные временные отчёты | PARTIAL: наследует неверные статусы |
| Modern/Wails bridge | generated typed bindings, реальные adapters | Все основные страницы подключены | 37 UI tests + Go service | сборка; desktop отдельно ниже | UNVERIFIED для полного E2E |
| GitHub Actions/release | CI, ZIP/offline/checksums | нет | исторический actionlint | GitHub runs отсутствуют | PARTIAL |
| Configuration | embedded YAML catalog/profiles, CLI overrides | выбор; нет полного редактора | strict parsing/DAG/catalog | локальные tests | PARTIAL |
| Legacy Windows 7 | сохранён отдельный модуль | Walk | исторические проверки | в этой сессии не запускался | FROZEN |

## Critical Findings

### P0-01 — ложный успешный результат после ошибки detection (исправлено)

`core/packages/install.go`, `Manager.runItem`: после exit 0 проверка `err == nil && !after.Installed` пропускала ошибку Detect и оставляла StatusSuccess. `TestInstallDetectionFailureCannotReportSuccess` переключает fake registry в ошибку после запуска установщика: до исправления получен `Status:success`, после — Failed/CodeVerifyFailed. Коммит `972e44e`. Реальный установщик не запускался. Reboot-ветка этим исправлением не меняется.

### P0-02 — требования профиля не применяются при выполнении (исправлено после аудита)

`core/inventory/applicability.go:GuardAdapter` реализован и тестируется, preview использует applicability. Однако `core/deploy/adapter.go:Precheck` не проверяет `Task.Requirements`; `deploy.Runner.Start/Resume`, Modern `deploymentAdapter` и CLI execution передают незавёрнутый Adapter. Проверка package catalog не заменяет ограничения профиля. Воспроизводимый по коду сценарий: windows-компонент с несовместимыми requirements всё равно достигает Execute. Нужна единая проверка свежего system snapshot на start/resume и тест, запрещающий вызов изменяющего адаптера. Это runtime safety gate, а не Legacy-only задача.

### P0-03 — reboot превращается в проверенный Success (software contract исправлен, VM acceptance открыта)

`core/deploy/adapter.go:Verify` возвращает nil при `rebootPending`. Engine трактует nil как Success и разрешает зависимые задачи (`core/engine/executor.go`). `core/engine/resume.go:retainResult` сохраняет Success, `ValidateForPlan` запрещает status RebootRequired; `deploy.Runner.Resume` отклоняет completed session без retryFailed. Даже retryFailed не перепроверяет Success. Поэтому «после reboot проверим» не реализовано. Требуется согласованная модель deferred verification, зависимостей, persistence и отчёта; локальная замена одного статуса небезопасна. До исправления нельзя обещать reboot continuation.

### P0-04 — неполные результаты компонентов выглядят успешными (частично исправлено)

`deploy.Adapter.executeBrowser/executeShare/executePrinter` отбрасывали Result.Warnings. `browsers.Manager.createWebApps` возвращает nil с warning при отсутствии браузера/ошибке ярлыка. `Verify` для browsers/windows возвращает nil без read-back. `precheckShare` завершает задачу AlreadyInstalled при наличии UNC даже если запрошенного ярлыка нет; printer precheck проверяет только имя, driver precheck — Hardware ID/status, а не целевой driver/version. Отчёт может показывать успех недостигнутого состояния.

Текущее состояние: required SMB/browser shortcuts имеют read-back и fail closed (`bab8e65`); warnings о неподтверждённых эффектах теперь сохраняются в result/session/report/UI (`e6148c2`, `4a03dd7`, `2fd8799`). Открыто: printer default/target read-back, browser/Windows policy read-back, driver target version/publisher.

### P0-05 — отсутствует общий контроль изменяющих операций (package/deployment исправлено после аудита)

`service/packages.go:StartPackageInstall` блокирует только packageState; `service/deployment.go:StartDeployment` — только deploymentState. Можно одновременно запустить установку через две страницы; engine resource keys действуют внутри одного run. `ResumeDeployment` проверяет running до Load/создания adapter и не перепроверяет под второй блокировкой: два конкурентных запроса могут запустить два writer одной сессии. Start также не перепроверяет closed после подготовки. Нужны единый operation coordinator, повторная атомарная проверка и тест конкурентных вызовов; не считать MaxParallel=1 глобальным mutex.

### P1-01 — TCP/IP printer port не создаётся

`core/printers/printers_windows.go:WindowsInstaller.AddPrinter` вызывает PrintUIEntry `/r IP_<address>`, но не создаёт TCP/IP port. В production-пути нет AddPort/XcvData или другого шага создания. Комментарий обещает создание порта; на чистой машине prerequisite отсутствует. Нужны создание/проверка порта, driver trust и VM-тест. После P0/MVP, функциональный приоритет принтеров P2.

## Technical Debt

- `core/profiles/profile.go` проверяет структуру, secret keys и OS tokens, но не параметры по категориям. `profiles/base.yaml` использует `check_ref: network-basic`, который `executeDiagnostics` не читает; вместо него выполняются дефолтные DHCP/gateway checks. Некорректный tcp endpoint там молча пропускается. Нужен семантический preflight до любой операции.
- `core/packages/install.go:runItem` подтверждает наличие приложения, но не достижение целевой версии после update. Старое приложение после exit 0 может считаться обновлённым. Нужен target-version/evidence контракт и regression test.
- `core/packages/runner_windows.go` использует CommandContext без Job Object/process-tree lifecycle. UI в `service/packages.go` заявляет, что запущенные установщики остановлены: для дочерних процессов это не доказано. Отмена не равна rollback.
- `deploy.Store.Load` читает весь файл до проверки 16 MiB, не проверяет EOF после JSON и равенство filename ID внутреннему ID. Список молча пропускает повреждённые файлы. `core/state/file.go` имеет более строгий bounded decoder, но deploy им не пользуется.
- Дублируется wiring managers в CLI, services и deploy; результаты standalone страниц богаче deployment task result. Перенос логики — только точечно по подтверждённым дефектам.
- `Service.GetJournal` и package/deploy events — отдельные ограниченные журналы в памяти. Engine скрывает raw error и сохраняет фазу; это защищает секреты, но недостаточно для диагностики. Нужны безопасные error codes/correlation, без raw stdout/password.
- `ExecutionPage.tsx` подавляет ошибки polling/list sessions; пользователь может не понять, что прогресс устарел. `Shell.tsx` settings button явно disabled: это известная отсутствующая возможность, не скрытая фиктивная кнопка.
- Поиск TODO/FIXME/HACK/NotImplemented/mock в production core/service/UI не выявил маркеров фиктивных реализаций; найденные placeholder относятся к HTML input. Отсутствие маркеров не отменяет конкретные дефекты выше. Полный dead-code/reachability анализ не выполнен; `GuardAdapter` — конкретный пример реализованного, но не подключённого execution gate.
- Frontend не имеет lint script/config; typecheck не заменяет lint. `go.mod` isolation сохранена; root dependency pin x/sys старее Modern осознанно, это само по себе не доказанная уязвимость.
- CI PowerShell multi-command steps не везде проверяют каждый LASTEXITCODE; успешная последняя команда может скрыть ранний сбой. `release.yml` не зависит от CI job и не обеспечивает документированные VM gates. Упаковка по-прежнему требует Legacy; это нужно развязать для Modern, сохранив Legacy workflow/artifacts.

## Missing Functionality

Относительно запрошенных сценариев отсутствуют или не закончены: безопасный отдельный elevated worker с сохранением пользователя; общий координатор операций; реальная VM-матрица установки/обновления/отмены; post-reboot verification; semantic profile preflight; полный результат ярлыков/default printer; persistent подробный безопасный журнал; конфигуратор профилей; гарантированное read-back настроек; TCP/IP port provisioning; доказанное offline E2E. Сетевой модуль диагностирует конфигурацию, но не меняет IP/DNS — это не реализованная настройка сети.

| Сценарий | Доказательство и предел |
|---|---|
| A Первый запуск | native Inventory tests реальны; frontend и bridge unit/service tests проходят; desktop smoke отдельно в Test Coverage |
| B Профили | YAML/наследование/DAG и UI tests; список задач структурный, семантика параметров и execution OS gate неполны |
| C Программы | GUI вызывает StartPackageInstall; unit/fake подтверждают pipeline; реальная установка через окно не проверена |
| D SMB | GUI → AddShare → CredMan/WNet/link; fake tests и native list/temp link; сервер/доступ к содержимому/credentials roundtrip этой сессией не подтверждены |
| E Принтеры | native enumeration подтверждена; чистая установка не готова из-за отсутствующего port creation; печать не выполнялась |
| F Диагностика | native DNS/local TCP/HTTP tests и error-path unit tests; GUI transport E2E ещё не подтверждён |
| G Восстановление | cancellation/checkpoint/resume unit и локальные file tests; реальная reboot boundary не реализована корректно |
| H Отчёт | HTML escaping/JSON/file output проверены; точность зависит от входных результатов, P0-03/04 остаются |

## Test Coverage

Среда: текущая Windows x64, установленный Go 1.26.2. Не устанавливались зависимости/toolchains. Полная Legacy-матрица намеренно не запускалась. Логи только в игнорируемом `local/audit-*.log`, не коммитятся.

- `go test ./... -count=1 -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip'` — PASS; `go vet ./...`, `go build ./...` — PASS. Исключены реальные записи в Credential Manager/registry, остальные native read-only и временные file tests выполнялись. Это не pristine VM.
- Новый `TestInstallDetectionFailureCannotReportSuccess`: FAIL до исправления (ложный Success); после исправления `go test ./core/packages ./core/deploy ./cmd/windeploy-packages -count=1` и соответствующий vet — PASS. gofmt и diff check выполнены.
- `npm test` — 37/37, 16 файлов; `npm run build` (strict tsc + Vite) — PASS. Lint НЕ запускался: script отсутствует.
- `scripts/build-modern.ps1` — итоговый PASS: bindings, 37 UI tests, frontend build, Modern Go tests/vet, module verification, EXE. Первый sandbox запуск упал при user32 init; второй — TEMP Access denied. Успех получен с TEMP/TMP/GOTMPDIR=`<repo>/local/audit-temp` и разрешённым запуском вне sandbox. Прямой ранний service test также падал на checkpoint в системном TEMP; полная сборка с workspace TEMP прошла. Не скрывать эту зависимость среды.
- Desktop: собранный EXE запущен с `--smoke-port 9223`; `node scripts/gui-modern-smoke.mjs` завершился exit 1: `Desktop WebView2 unavailable`. CDP/DOM/bridge проверки не достигнуты. Причина именно этого запуска не локализована; исторический 0x8000FFFF не выдаётся за новую диагностику. Сам факт EXE build не доказывает UI runtime.
- Native установки, SMB server, print job, policies/settings, Windows 10/LTSC VM, reboot, race detector и GitHub-hosted CI не подтверждены этой сессией. Test coverage измеряет выполняемые ветки, не production readiness.

## Security Findings

Подтверждённые меры: elevated token prechecks для требующих прав managers; bounded transfer/temporary cleanup и SHA-256; Authenticode; schema secret-key/URL-userinfo rejection; пароль SMB передаётся CredMan/WNet в памяти, не argv; HTML escaping и исключение task parameters из отчёта; localhost debug только opt-in. Это не полная security certification.

| Риск | Доказательство | Мера |
|---|---|---|
| Высокий: другой пользователь после Run as administrator | service/packages предлагает перезапуск; browsers RegistryStore использует HKCU, DesktopDirectory/CredMan — текущий процесс; исходный SID не передаётся | elevated worker и явный user/machine context; VM two-account test до production |
| Высокий: обход profile applicability/одновременные mutations | P0-02/P0-05 | gate всех start/resume и общий coordinator |
| Высокий при недоверенном каталоге сессий | deploy.Store хранит полный план; 0600 не является Windows ACL, hash связывает план, но не аутентифицирует автора; CLI позволяет выбрать storage | trusted per-user ACL, owner/context validation перед elevated resume; bounded parsing |
| Средний: trust не одинаков для всех источников | verifySignature пропускает Signer=nil при sha256 policy; publisher проверяется substring; WinGet policy отдельно; WindowsSigner отключает revocation | определить явные политики доверия, точное publisher matching, tests missing verifier; не заявлять универсальное SHA+signature |
| Средний: изменяемый artifact между verify/run | resolveArtifact проверяет файл по пути, runner открывает его позже; кэш не закреплён handle/ACL | trusted cache/ACL и защита от замены, проверить в VM |
| Средний: HTTPS redirect downgrade | HTTPFetcher проверяет исходный scheme, NewHTTPFetcher не задаёт CheckRedirect для scheme конечного URL | запрет downgrade/userinfo redirects с HTTP тестом; hash снижает integrity risk, но не transport gap |
| Средний: command resolution/отмена | WindowsRunner использует LookPath/cmd fallback, settings/printers вызывают системные exe по имени; нет process tree control | absolute trusted system paths, строгие args и lifecycle tests; exploit не демонстрировался |
| Средний: SMB disconnect и сохранение credentials | Manager.Add сохраняет password до успешного connect; ResolveConflict принудительно отключает соединения сервера | preview списка/подтверждение и восстановление credentials при ошибке; VM server tests |
| Средний: driver/printer verify недостаточен | driver success по Hardware ID/status; printer route не использует drivers.INFVerifier | verify target driver/version, trust и port, затем VM |

По выборочному анализу подтверждённой утечки пароля в production logs не обнаружено. Это не гарантия: строковые credentials остаются в памяти Go/React, task parameters могут содержать нераспознанные секреты, startup logger пишет runtime error. Shell injection не объявляется доказанной: аргументы передаются массивами, catalog фильтрует метасимволы; cmd fallback требует отдельного негативного теста. Session IDs ограничены regexp, имена shortcut sanitised; arbitrary trusted local source paths являются функцией, а не автоматически path traversal.

## Recommended Roadmap

1. P0: runtime applicability, честные pending/reboot/results, единый mutation lifecycle и user-context gate. Сначала unit/service regression, затем ограниченный read-only desktop smoke.
2. P1: установка/обновление одного разрешённого пакета в чистой Windows 11 VM; detection target version, UI выбор/прогресс/отмена, логи и отчёт с фактическим состоянием. Затем Windows 10/LTSC.
3. P1: semantic profiles и SMB shortcut/credentials с тестовым сервером; диагностика, safe persisted error codes и отчёт. До окончания этих сценариев не расширять сложные функции.
4. P2: printer ports/trust, driver target verification, browser/settings read-back, reboot/offline end-to-end. Уже имеющийся код сохраняется, но не считается готовым.
5. P3: UX, performance, дополнительные профили, редактор и документация. Legacy — отдельное FROZEN; возвращение только по решению владельца.

Конкретная следующая итерация с критериями и командами — в `TASKS.md` и `HANDOFF.md`.

### Итоговое измерение покрытия
После исправления: root go test ./... -count=1 -cover с теми же двумя исключениями — PASS вне sandbox с workspace TEMP. Sandbox попытка падала на DLL initialization в native COM/KnownFolder tests. Покрытие statements: engine 96.6%, profiles 93.8%, inventory 83.6%, network 82.2%, state 80.7%, reporting 78.3%, drivers 76.3%, packages 63.6%, settings 62.7%, printers 56.1%, shares 50.9%, browsers 50.0%, deploy 43.0%. Низкое deploy coverage и отсутствие изменяющего E2E остаются пробелами; это не общий процент production-готовности.
