# WinDeploy — Agent Handoff

## Состояние проекта
Текущий этап: 3 — Inventory (AUTONOMOUS DEVELOPMENT).
Текущая задача: завершены 9/10 задач: system identity, CPU, RAM, volumes, interfaces, IP config, apps, collector, applicability; далее CLI и full-project verification.
Последний завершённый этап: базовые критерии этапа 2 — Profiles (YAML/наследование/примеры формируют валидный план); ядро этапа 1 расширено retries/concurrency/state/resume.
Текущая ветка: master. Последний коммит реализации: adef541 — `feat(profiles): build selected plans and add workstation examples`.
Пользователь разрешил MULTI-TASK по 5–10 задач, отдельные коммиты и переходы этапов. Это выше прежнего ограничения одной задачи; не запускать несколько агентов без отдельного указания. AGENTS обновлён.

Десять функциональных коммитов сессии:
1. dfdadde — dry-run preview.
2. 0626c5e — opt-in retries.
3. de54b7a — bounded concurrency/resource keys.
4. c3085b0 — file snapshots.
5. 4e7fb24 — resume.
6. f1a7ec8 — checkpoints/fail-closed storage.
7. b0ca645 — profile model/validation.
8. f4ea631 — strict YAML.
9. 0863f13 — inheritance.
10. adef541 — selected plans/clone/examples.
Промежуточные docs checkpoint-коммиты: 5151f06, 5d3bd45, 3bebe07, 14db356. Итоговый docs-коммит — HEAD после завершения (см. git log).

## Что уже реализовано
- Inventory Native.System/CPU/Memory — read-only Windows API/registry, build tags; чистые модели и unit/native tests. System использует RtlGetVersion, native architecture (WOW64-aware); CPU model/vendor и GetActiveProcessorCount across groups; GlobalMemoryStatusEx для RAM. x/sys v0.6.0 (upstream go 1.17) закреплён для Go 1.20-совместимых Windows wrappers. Реальная Win7 VM пока не проверялась.
- Native.Volumes: roots/drive type + GetDiskFreeSpaceEx (64-bit, quota-aware) для fixed/ramdisk; remote/removable/optical только metadata, capacity явно unknown. Partial failures сохраняют доступные тома. Native.Interfaces: IPv4/IPv6 prefixes/flags/MAC/MTU, без вывода о reachability. Native.NetworkConfig: GetAdaptersAddresses, gateways/DNS/DHCPv4/IPv4 enabled/link speeds/operational state, IPv6 scopes сохранены; bounded native buffers и record length checks.
- Native.Apps: видимые HKLM/effective-user HKCU Uninstall entries в native 32/64 views; dedup shared views, без UninstallString/InstallLocation/Win32_Product. AppX/MSIX/portable apps не покрыты. Apps scope означает пользователя процесса, не автоматически интерактивного пользователя при elevation.
- inventory.Collect/Report: семь секций, explicit ok/partial/failed/cancelled/not-collected; independent failures продолжают сбор, cancellation останавливает. ErrIncomplete и Complete=false не дают выдать частичный отчёт за полный; raw errors не сериализуются.
- inventory.Applicable/GuardAdapter: fail-closed unknown OS/arch, canonical profile tokens, native OS arch вместо process arch; все три фазы guarded, несовместимый Precheck => NotApplicable, dependents => Skipped. Empty requirements разрешают только поддерживаемую workstation matrix, catalog должен сузить для конкретного пакета.
- engine: Task/TaskResult/статусы, Validate, OrderTasks, Run с Adapter Precheck/Execute/Verify, отмена/timeout, пропуск заблокированных зависимостей.
- PreviewPlan: независимые метаданные без Parameters/фиктивных успешных результатов, scope structure-and-dag-only.
- RunWithOptions: MaxAttempts/RetryDelay (только RetryableError=true, новый Precheck), MaxParallel (default 1), ResourceKey для взаимного исключения всех фаз/ожиданий. WindowsInstallerResource обязателен для будущих MSI-адаптеров.
- Snapshot/NewSnapshot/Validate/ValidateForPlan: версия, SHA-256 точного Pending-плана, результаты без Parameters и сырого текста ошибок. Проверка ID/зависимостей до resume.
- core/state.FileStore: JSON до 4 MiB, строгая загрузка, temp+Sync+Close+Rename. Не обнуляет старый checkpoint до успешной записи.
- Resume: сохраняет Success/AlreadyInstalled/NotApplicable/намеренные Skipped; Failed требует RetryFailed; Running/Cancelled/зависимые Skipped проходят Precheck. RebootRequired flag и записанные Attempts накапливаются.
- RunOptions.Store: initial snapshot, Running до запуска, финальный результат после операции; ошибка сохранения => ErrCheckpoint, отмена активных и запрет новых задач. Сохранение при отмене через отдельный 5s context.
- profiles schema v1: модель/валидация; Load/Marshal strict YAML до 1 MiB; Single-parent Resolve с циклами/лимитом 64; Clone; BuildPlan с ручным выбором и closure зависимостей; Preview.
- Восемь YAML-примеров: base, office, accounting, reception, warehouse, service, administrator, legacy. Все проверены до engine.PreviewPlan, реальных установщиков/серверов/credentials нет.
- docs/architecture.md, docs/execution.md, docs/profiles.md описывают публичные API и ограничения.

## Что проверено
- Текущий пакет: go test ./core/inventory/... и go vet ./core/inventory/... PASS после каждой из 9 задач; дополнительно go test ./core/engine/... для guard integration. Реально прочитаны все семь секций на текущей Windows, aggregate Complete=true. Приватные значения не выведены в журнал. Native layouts/short records/IPv6 scopes/partial errors/cancellation и engine guard проверены.
Go 1.26.2, доступная Windows:
- windows/amd64: `go test ./...`, `go vet ./...`, `go build ./...` — PASS.
- windows/386: `go test ./...`, `go build ./...` — PASS; это не проверка Windows 7.
- `go mod verify` — all modules verified; go mod tidy выполнен.
- gofmt и git diff --check — PASS.
- Тесты после каждой функциональной задачи. Concurrency проверяет реальное перекрытие/лимиты/ресурсы/ожидание отмены через каналы. Checkpoints: ошибки initial/running/completion Save, очистка активного адаптера, сохранение при отмене. FileStore restart + fake installation проверяет отсутствие повторной установки.
- Profiles: malformed/unknown/duplicate YAML, aliases/merge keys, размеры, secret errors, inheritance/cycles/isolation, selection/disabled/missing deps, cloning и восемь примеров.
- Race detector не запускался: gcc/clang не найдены.

Команды окружения (не меняют глобальные настройки):
```powershell
$env:GOCACHE = Join-Path $PWD 'cache\go-build'
$env:GOMODCACHE = Join-Path $PWD 'cache\_go-mod'
go test ./...
go vet ./...
go build ./...
```
Для проверки x86 добавить `$env:GOARCH = '386'`; после неё убрать переменную в той же оболочке, если продолжаете amd64-проверки.
Старый cache/go-mod был перенесён в cache/_go-mod: без подчёркивания `go test ./...` захватывал исходники скачанной test-зависимости без go.mod. Кэш исключён Git, исходники проекта не затронуты.

## Что сейчас не работает
- Нет Inventory, GUI, реальных системных адаптеров/каталога пакетов. Примеры профилей описывают намерения, не готовые установочные действия.
- Go 1.20 toolchain и VM-матрица Windows 10/11/7 SP1 x86/x64 не проверены. x86-тесты на текущей ОС не подтверждают Win7 runtime.
- Отмена кооперативная: Adapter и Store должны соблюдать context, иначе Run ждёт. Не оставляет фоновые операции после возврата.
- Snapshot хранится в доверенном каталоге, не аутентифицирован; SHA-256 — привязка плана, не защита от подмены. Завершённые результаты не перепроверяются по live inventory.
- FileStore: один процесс-владелец пути, mutex защищает только экземпляр. Каталог создаёт вызывающий код и защищает Windows ACL; mode 0600 не заменяет ACL. Power-loss atomicity/durability зависит от filesystem и не гарантирована.
- Checkpoint не делается между фазами и retry attempts, поэтому после внезапного завершения счётчик попыток может быть неполным; interrupted всегда проходит Precheck.
- Обязательный reboot barrier/boot-ID/автозапуск после перезагрузки не реализованы. Resume отклоняет status RebootRequired; флаг после проверенного Success поддерживается.
- Полное маскирование произвольных секретов и detailed diagnostic logging отсутствуют. Engine пишет только известные этапы ошибок. Profiles блокирует распространённые secret keys/URL userinfo, но не распознаёт секрет в произвольном значении.
- GitHub CLI отсутствовал; remote/CI/releases не создавались.

## Незавершённая работа
Незавершённых исходников нет. Следующая работа — read-only Inventory. GUI редактора/импорта/экспорта профиля появится на этапе 4; backend API уже работает. Не считать этапы Inventory/GUI выполненными.

## Архитектурные решения
- Общее ядро: go 1.20, без UI/Windows-команд. Единственная прямая зависимость — go.yaml.in/yaml/v3 v3.0.4 (upstream go 1.16; https://github.com/yaml/go-yaml/blob/v3.0.4/go.mod). go.mod/go.sum зафиксированы.
- Task декларативен; Adapter не меняет вложенные maps/slices и должен быть thread-safe при MaxParallel>1. Только Success/AlreadyInstalled удовлетворяют зависимости. Success + RebootRequired означает подтверждённую Verify готовность.
- Run принимает только Pending. Ошибки задач читать из results, error — invalid input/ctx/ErrCheckpoint. Resume требует точный исходный Pending-план. RetryFailed и RetryableError — разные разрешения: первое для сохранённого Failed, второе для повторов внутри запуска.
- OrderTasks O(V²+E), tie-break по исходному индексу. Concurrent completion недетерминирован, results упорядочены. Общий windows-installer ResourceKey задаёт вызывающий код, движок не угадывает тип установщика.
- Profiles: полный override компонента по ID на позиции родителя; новые добавляются, disabled — tombstone только с ID. YAML anchors/aliases/merge keys запрещены. Resolve не читает filesystem/network. BuildPlan(nil selection) — все enabled; пустой ненулевой selection — ни одного; зависимости включаются автоматически.
- OS tokens профилей: windows-7-sp1/windows-10/windows-11; arch 386/amd64. Inventory должен использовать согласованное представление и не подменять реальные сведения предположениями.

## Следующая задача
Добавить cmd/windeploy-inspect: read-only inventory (text/JSON) и preview profiles; exclusive output creation, exit codes для partial/cancelled/errors. Затем go test/vet/build ./... на amd64, test/build на 386, реальный CLI smoke, dependency verify и HANDOFF. После 10 задач не расширять пакет до GUI без общей проверки.

## Инструкция следующему агенту
1. Проверить status/log, прочитать AGENTS/TASKS и этот HANDOFF. Затем docs/architecture.md и только относящиеся engine/profile требования ОС.
2. Создать core/inventory с моделью SystemInfo (hostname, version/build, architecture) и маленьким интерфейсом read-only provider для тестов. Windows-specific чтение изолировать build tags, сохранить компиляцию общего ядра на Go 1.20. Проверить API по первичным документам, включая Win7 SP1; не полагаться на GetVersionEx без учёта manifest/version lie.
3. os.Hostname и проверенный способ чтения версии должны возвращать реальные данные; неизвестную версию показывать ошибкой/unknown, не угадывать. Не запускать PowerShell с административными изменениями.
4. Unit-тесты данных/ошибок/fake provider; реальная read-only проверка на доступной Windows. Не называть это доказательством Win7. Выполнить go test ./core/inventory/... и go vet/build ./... с локальными кэшами.
5. Коммит `feat(inventory): detect Windows version and hostname`; обновить TASKS/HANDOFF. В MULTI-TASK далее брать CPU/RAM/disks/network/apps небольшими задачами, не начинать весь GUI преждевременно.

## Чего нельзя делать
Не устанавливать Wails/драйверы/программы и не изменять сеть/защиту на хосте. Не выдавать fake/integration-тест за реальную установку или Win7-совместимость.
Для записи .git требовался escalated exec с `git -c safe.directory=C:/Users/ashtr/OneDrive/Desktop/git/WinDeploy ...` из-за владельцев sandbox/user. Не менять глобальную Git-конфигурацию/identity и не коммитить cache/binaries/runtime state/credentials.
