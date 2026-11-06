# WinDeploy — Agent Handoff

## Состояние проекта
Текущий этап: 4 — Modern GUI, текущая сессия в работе.
Текущая задача: Modern 1–6 завершены. Далее real dashboard, detail tables, Profiles/DAG/applicability, journal и финальный desktop smoke.
Последний завершённый этап: 3 — Inventory (реальные данные текущей Windows + tests; целевые VM-проверки отдельно).
Последний коммит реализации: 1ff69d4 — feat(cli): add read-only inventory and profile preview commands.
Текущая ветка: master. Предыдущая база: 1bc74b6.
Пользователь разрешает пакеты 10–15 задач, отдельные коммиты/тесты и автоматические переходы этапов. После 2–3 задач обновлять HANDOFF; после 10–15 проверять проект целиком. AGENTS обновлён. Делегация не запрошена.

Итоговый отчёт с 10 коммитами, проверками и ограничениями: [docs/reports/autonomous-inventory.md](docs/reports/autonomous-inventory.md).

## Что уже реализовано
- desktop/modern — изолированный Go 1.26 модуль с Wails v3.1.0 stable и local replace на root; root go.mod остаётся go 1.20. React 19.3.1/TS 7.0.2/Vite 8.3.6/runtime 3.1.0 pinned, UI tests happy-dom (совместим с Node 24.13; jsdom latest требовал более новый Node).
- Настоящий EXE bin/WinDeploy-Windows-x64.exe запущен через Wails/WebView2 156; проверены HWND/Responding и DOM через CDP, screenshot local/modern-bootstrap.png. GUI пока только стартовая страница, не dashboard. scripts/gui-bootstrap-smoke.mjs. Для background GUI держать PowerShell WaitForExit, иначе tool host закрывает child после завершения exec. Smoke CDP localhost :9223 opt-in, proxy env отключать только в процессе smoke.
- scripts/test-core.ps1 проверяет root test/vet/build amd64+386 отдельным Go executable. Официальный Go 1.20.14 download оборвался, checksum не прошёл; непроверенный архив не запущен. Resume curl в фоне; нельзя заявлять Go 1.20 PASS до actual test. docs/core-go120.md и docs/modern-compatibility.md.
- Go 1.20.14 после curl resume/SHA-256 получен и проверен: test/vet/build root PASS на amd64 и 386. Toolchain перенесён в local/_toolchains (исключает его test/src из root ./...), отдельный cache/go-build-go120. Это не Win7 VM proof. Root go.mod не повышался.
- UI design system, shell/navigation, disabled future modules, доступные controls/unknown progress/empty states; UI/TS/build PASS.
- Modern service: actual inventory.Native provider, async Collect, один scan одновременно, CancelScan, partial/errors, ring journal реальных операций и ServiceShutdown. Wails CLI v3.1.0 generated TS interfaces/bindings; useInventory polling только во время сканирования, без fixture fallback. Go service tests + frontend tests/typecheck/build PASS. Dashboard/detail/profile views ещё не реализованы.
- core/engine: Task/Result/status, DAG/OrderTasks, Pending-only Run, Adapter lifecycle, cancellation/timeout, PreviewPlan, opt-in RetryableError/attempts, MaxParallel и ResourceKey.
- core/state + engine snapshots/resume: JSON checkpoints, точный plan hash, fail-closed storage errors, interrupted => Precheck, Failed только через RetryFailed. Восстановление не дублирует сохранённые Success/AlreadyInstalled.
- core/profiles: strict bounded YAML schema v1, single-parent inheritance/cycles, Clone, selected dependency closure, Pending plans, safe preview; 8 YAML примеров.
- core/inventory/System: hostname, RtlGetVersion build/SP/product type, canonical OS tokens, GetNativeSystemInfo native architecture отдельно от runtime.GOARCH.
- CPU: model/vendor из read-only registry, GetActiveProcessorCount across groups; RAM: GlobalMemoryStatusEx 64-bit usable physical counters.
- Volumes: logical roots/type, fixed/ramdisk capacity/available bytes; остальные типы capacity_known=false. Interfaces: IPv4/IPv6 prefixes/flags/MTU/MAC. AdapterConfig: gateways/DNS/DHCPv4/link speeds/operational state/IPv6 scope через GetAdaptersAddresses.
- Apps: видимые Uninstall entries HKLM/effective-user HKCU в 32/64 views, dedup shared entries. Никаких Win32_Product/UninstallString/исполнения команд.
- Collect/Report: 7 секций с ok/partial/failed/cancelled/not-collected; raw provider errors не экспортируются. Complete=false + ErrIncomplete для частичного сбора. Независимые ошибки не останавливают остальные секции, cancellation останавливает.
- Applicable/GuardAdapter: сравнение требований с native OS/arch, fail-closed unknown, NotApplicable для mismatch; все три фазы guarded. Проверено совместно с engine.Run.
- cmd/windeploy-inspect: read-only inventory text/JSON и profile preview. Exit 0 complete, 1 input/output/error, 2 partial, 3 cancelled/deadline. Output создаётся только новым файлом; previews не вызывают Adapter и не показывают Parameters.
- README и docs/inventory.md описывают источники и границы; docs/architecture/execution/profiles — контракты предыдущих модулей.

## Что проверено
Go 1.26.2 на доступной Windows:
- После каждой задачи — относящиеся unit/native tests и vet; реальные read-only Windows API/registry разрешены текущим объёмом и отдельно отмечены в AGENTS.
- windows/amd64: go test ./..., go vet ./..., go build ./... — PASS.
- windows/386: go test ./..., go build ./... — PASS; тесты реально выполнены на текущей x64 Windows.
- linux/amd64 cross-build go build ./... — PASS (без runtime-проверки).
- go mod verify — all modules verified; gofmt и git diff --check — PASS.
- Реальные EXE в bin/: x64 и x86 inventory Complete=true/7 sections, 3 volumes/7 interfaces/167 visible app entries. Build совпал с реестром, hostname — с Environment.MachineName. x86 EXE верно показал native=amd64/process=386 и те же 167 apps.
- CLI preview всех 8 профилей через реальный EXE — PASS.
- Private full JSON smoke-report только в local/ (игнорируется Git), без вывода hostname/IP/MAC/названий программ в общий отчёт.
- Race detector не запускался: gcc/clang не найдены в PATH.

Локальные кэши и базовые команды:
```powershell
$env:GOCACHE = Join-Path $PWD 'cache\go-build'
$env:GOMODCACHE = Join-Path $PWD 'cache\_go-mod'
go test ./...
go vet ./...
go build ./...
go mod verify
```
GOARCH=386 использовать только для отдельной проверки, затем убрать в той же оболочке. Для cross-build Linux выставить GOOS=linux/GOARCH=amd64 только в отдельном процессе.
Не переносить module cache в обычный cache/go-mod: исходники скачанных пакетов без собственного go.mod захватываются `./...`; подчёркивание в _go-mod исключает их.

## Что сейчас не работает
- Нет Modern/Legacy GUI и реальных изменяющих систему adapters/catalog/installers. CLI — диагностика и preview, не полноценный WinDeploy installer/release.
- Go 1.20 toolchain и целевая VM-матрица Windows 10/11/7 SP1 x86/x64 не проверены. Текущие EXE собраны Go 1.26.2 и не являются Win7 Legacy-сборками.
- Inventory: CPU — первая brand/vendor + logical count, не полный socket/hybrid topology. RAM usable, не DIMM labels. Volumes logical/quota-aware, без physical model/SMART. Remote/removable/optical capacity намеренно не запрашивается.
- Apps только видимые Uninstall entries: нет AppX/MSIX/portable apps и проверки файловой исправности. HKCU — пользователь процесса; при elevation не impersonate интерактивного пользователя.
- Network не выполняет reachability/DNS-resolution/SMB/TCP/ICMP и не читает leases DHCPv6/route table. Up/Operational не доказывает доступность сервера. ARM emulation у GetNativeSystemInfo имеет ограничения; arm64 выполнение не поддерживается.
- RtlGetVersion игнорирует обычный manifest version lie, но application compatibility layer может влиять на ответ; текущий build перепроверен по реестру только в smoke-test.
- Синхронные native API не прерываются принудительно; context проверяется вокруг вызовов/между элементами. Adapter/Store также обязаны соблюдать context.
- GuardAdapter должен подключать вызывающий код перед реальными task operations; прямой engine.Run не читает ОС автоматически. Empty requirements не заменяют проверку конкретного пакета/драйвера.
- State snapshots доверенные локальные, не аутентифицированы и не перепроверяют live inventory старых Success. Одна session path принадлежит одному процессу; FileStore mutex только на экземпляре; Windows ACL задаёт администратор. Power-loss atomicity/durability не гарантированы. Attempts могут быть неполными при crash между фазами/retries.
- Обязательный reboot barrier/boot-ID/автозапуск после reboot отсутствуют; status RebootRequired при Resume отклоняется, verified Success+reboot flag поддержан.
- Engine diagnostics только безопасный этап. Полного маскирования произвольных секретов нет; profiles запрещает частые secret keys/URL userinfo, но не распознаёт секрет в произвольном значении.
- GitHub CLI отсутствовал; remote/CI/releases не создавались.

## Незавершённая работа
Незавершённых исходников нет. Следующая работа — первый ограниченный шаг Modern GUI. Не считать его выполненным по наличию CLI. Не расширять эту завершённую сессию новыми задачами; сначала зафиксировать итоговую документацию.

## Архитектурные решения
- Root module windeploy, go 1.20. Закреплены go.yaml.in/yaml/v3 v3.0.4 (upstream go 1.16) и golang.org/x/sys v0.6.0 (upstream go 1.17). Это не доказательство runtime Win7.
- Windows API/registry изолированы build tags, общие модели/Provider/Collector переносимы; unsupported OS-provider возвращает ErrUnsupported, не фиктивные данные.
- inventory импортирует engine для GuardAdapter; engine не импортирует inventory и не зависит от Windows. UI должен вызывать сервис, без бизнес-логики внутри React.
- Сохранять root Go 1.20 при добавлении современного Wails/toolchain. Modern UI — отдельный Go-модуль; не обновлять root go.mod ради UI. Отдельно проверять сборки вложенного модуля, поскольку root ./... его не включает.
- Task декларативен, Adapter не меняет вложенные maps/slices и thread-safe при MaxParallel>1. Общий WindowsInstallerResource назначает adapter/catalog. RetryableError разрешает безопасные повторы внутри запуска; RetryFailed — повтор сохранённого Failed. Только Success/AlreadyInstalled удовлетворяют зависимости.
- Profile override по ID целиком, disabled tombstone только ID/disabled; nil selection — все enabled, пустой ненулевой — ни одного. Canonical OS tokens windows-7-sp1/windows-10/windows-11; arch 386/amd64. Resolve не делает скрытого IO.

## Следующая задача
Текущая сессия: Modern 4 — UI tokens/primitives, затем navigation shell и typed real service. Завершить 10 задач текущего user prompt. Modern 1 повторно проверить при готовности SHA-verified local toolchain. Final HANDOFF переписать по фактам, старые разделы ниже пока описывают предыдущую сессию.

## Инструкция следующему агенту
1. status/log, AGENTS/HANDOFF/TASKS; затем docs/architecture.md и docs/inventory.md. Не исследовать всё ядро заново.
2. Проверить официальные Wails docs/releases и установленный Node/npm/Go; v3 использовать только если фактический статус/совместимость позволяет. Выбор версии/ограничения записать коротко, без переархитектуры ядра. Node/npm найдены в PATH, версии не проверялись.
3. Создать изолированный Modern Go-модуль (например desktop/modern), локальный replace на root core. Подключить Inventory Collect и Profiles Preview через небольшой сервис; не переносить Windows APIs или выполнение tasks в UI. Ошибки/partial/unknown отражать честно.
4. Затем React/TypeScript dark dashboard на русском, реальные данные/выбор профиля/план. Не показывать фиктивный Success. Проверить WebView2 dependency и корректный запуск на доступной Windows, сборку UI и DPI/1366x768 насколько позволяет среда.
5. Каждая законченная часть — tests/build, отдельный commit, TASKS; HANDOFF после 2–3 задач. После 10–15 обязательны whole-project + nested Modern checks/smoke. Core go test/vet/build и x86 не ломать.

## Чего нельзя делать
Не менять сеть/защиту, не устанавливать сторонние программы/драйверы на хосте. Реальные изменяющие систему API/установки — только VM. Не заявлять Win7 или всю VM-матрицу по текущим read-only tests.
Для записи .git требовался escalated exec с `git -c safe.directory=C:/Users/ashtr/OneDrive/Desktop/git/WinDeploy ...` из-за sandbox/user владельцев. Не менять глобальную Git-конфигурацию/identity, не коммитить binaries/cache/private reports/state/credentials.
