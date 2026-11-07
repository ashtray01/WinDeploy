# WinDeploy — controlled development

## Текущий этап

2026-11-07: пользователь сообщил об успешном запуске; исправлены diagnostic log/UTF-8, добавлен user-context gate, обязательные ярлыки больше не дают ложный Success; в этой итерации warnings о неподтверждённых эффектах сохраняются в session/report/UI. Root/Modern checks и actual GUI smoke PASS на текущем Win11 (warnings-итерация: build/tests, без нового GUI smoke); VM/Win10/LTSC не подтверждены. Полный `go test ./...` в workspace нестабилен — см. HANDOFF. Источники — AUDIT/HANDOFF.

## Завершено в этой итерации

- [x] P0-04 structured warnings: `engine.TaskResult.Warnings` + optional `WarningReporter`, sanitize/bounds/Validate, перенос warnings из shares/printers/browsers, JSON+escaped HTML отчёт, Modern DeployTaskView.warnings и UI (`e6148c2`, `4a03dd7`, `2fd8799`). Negative tests PASS; UI 41/41; Modern build PASS. Пакет остановлен после этой задачи по решению владельца.

Ранее в текущем этапе: diagnostic launcher (`a82f11e`), P0-06 user-context gate (`c9e1839`), обязательные SMB/browser shortcuts (`bab8e65`). Исторические результаты не заменяют текущую VM acceptance.
## P0 — блокирующие ошибки

- [ ] Modern startup matrix: пользователь подтвердил запуск, agent actual GUI/DOM/bridge smoke PASS с explicit UDF/log-dir. Default UDF и Windows 10/LTSC отдельно, старый профиль/ACL не сбрасывались.

- [x] P0-02: применять требования ОС/архитектуры профиля на start/resume (core/deploy, CLI, service). Готово: несовместимая или неизвестная система не достигает изменяющего adapter; свежий inventory для resume; unit + CLI/service tests.
- [ ] P0-03 VM acceptance: программный контракт исправлен (`d4c4171`), pending не Success/Completed; Verify-only после смены kernel BootID. Unit/file/CLI/service/UI tests PASS. Осталось: реальный reboot/fast startup + native readiness/dependencies/report в Win11 VM, затем Win10/LTSC.
- [ ] P0-04 remainder: SMB/browser required shortcuts исправлены (`bab8e65`), structured warnings сохраняются (`e6148c2`/`4a03dd7`/`2fd8799`); нужны printer default/target read-back и browser/Windows registry read-back. Исторические Success не перепроверяются автоматически.
- [x] P0-05: единое исключение изменяющих операций и атомарный start/resume/shutdown. Готово: package page и deployment не запускают конкурирующие установки; два resume не пишут одну сессию; barrier-based concurrent service tests.
- [ ] P0-06 VM acceptance: software fail-closed process/shell SID gate реализован (`c9e1839`). Two-account/Run as another admin/RDP/headless, same-user UAC и scope tests в VM ещё нужны. Worker/исходный SID в IPC и session owner authentication отсутствуют.

## P1 — рабочий MVP

- [ ] Подтверждать target version после update, отличать unknown/absent/outdated; fail/cancel detection tests. Каталог и detection уже реализованы, переписывать их не требуется.
- [ ] Semantic profile preflight: category parameters, package refs, check_ref/diagnostic targets, dependency closure; ошибочный профиль отклоняется до первой мутации. Preview и execution используют одинаковые правила.
- [ ] Modern desktop acceptance: actual EXE DOM/bridge/native Inventory smoke PASS с explicit UDF; проверка запускалась Hidden, MainWindowHandle=0. Нужна visible/default-user проверка, Windows 10/LTSC отдельно.
- [ ] Win11 VM: один одобренный пакет (7-Zip), clean install → detection → повтор без установки → update/failure/cancel; UI progress, exit codes и отчёт совпадают с системой. Затем остальные пакеты каталога и Windows 10/LTSC.
- [ ] SMB VM/server: UNC access, credential storage в правильном контексте, conflict preview/consent, shortcut existence, повтор и ошибки. Native WNet/list/link код уже есть; полный сценарий не подтверждён.
- [ ] Persistent безопасный журнал с error codes и связью task/session; не включать secrets/raw installer output. UI сообщает об ошибках polling; отчёт сохраняет warnings/partial outcomes (warnings уже сохраняются, журнал — нет).
- [ ] Защитить sessions/cache Windows ACL, проверять владельца и bounded JSON перед elevated resume; отрицательные parser/path tests и VM ACL verification.
- [ ] CI/Modern release: fail-fast для каждой native команды, отдельные Modern gates без Legacy dependency; GitHub run после появления remote. Публикация только после выполнения критериев и явного задания владельца.

## P2 — расширенные возможности

- [ ] Drivers: подтвердить целевую INF/version/publisher вместо одного status=ok; VM install/reboot verification.
- [ ] Printers: создание TCP/IP port, INF trust, queue/port/model/default read-back, контролируемая тестовая страница в VM/тестовом принтере.
- [ ] Browsers/Windows: read-back, сохранение старых значений и контролируемое восстановление после частичного сбоя; user/machine scope в VM.
- [ ] Offline bundle: автоматизировать проверку файлов, trust, отсутствие сети и реальную установку в чистой VM; существующие ZIP не доказательство.
- [ ] Полное reboot continuation/boot identity и opt-in запуск после входа — после устранения P0-03, без ложных обещаний текущей реализации.
- [ ] Process-tree cancellation и безопасное ожидание installer children; не обещать rollback после kill.
- [ ] Trust hardening: HTTPS redirects, exact signer policy, missing verifier, trusted system executable paths и защита artifact от подмены между verify/run.

## P3 — улучшения

- [ ] Frontend lint с минимальной конфигурацией и отдельным CI gate (сейчас lint script отсутствует).
- [ ] Единый источник версии (bundle/frontend/User-Agent), документация deployment profiles и package sources.
- [ ] Редактор профилей, дополнительные рабочие места и конфигурации после semantic schema.
- [ ] UX, accessibility, производительность по измерениям; расширенная диагностика после основного MVP.
- [ ] Проверка актуальности/CVE dependencies без автоматического массового обновления; root Go version не повышать автоматически.

## FROZEN / DEFERRED — Windows 7

Существующие `legacy/`, `profiles/legacy.yaml`, tests, scripts и docs сохранены. Возвращение только по отдельному решению владельца. Ни один пункт ниже не является gate Modern MVP.

- [ ] Windows 7 SP1 x86/x64 VM, Legacy selftest/smoke/установки/SMB.
- [ ] Новые функции Legacy GUI, Legacy-only fixes/tests и адаптация драйверов для Win7.
- [ ] Legacy-specific релизы, расширенная матрица старых toolchains/ОС.

Исторически Legacy Walk x86/x64 собиралась и окно проверялось на Windows 11; root проверялся Go 1.20.14. Это не Win7 runtime и не новый результат этой сессии. Сохранять достигнутую совместимость, если она не мешает Modern; новые несовместимые зависимости изолировать в Modern module.

## Следующая итерация — 4 задачи

| Задача | Приоритет / компоненты | Критерий готовности / проверка |
|---|---|---|
| 1. Session trust/parsing | P1 security | bounded decoder до allocation, trailing JSON/ID equality, затем Windows ACL/owner перед elevated resume; tests и VM ACL verification отдельно |
| 2. Read-back остаточных эффектов | P0-04 remainder | printer default/target и browser/Windows policy читаются обратно; warnings уже сохраняются — нужен именно read-back; отрицательные tests; без новых printer features |
| 3. Стабильность полного test-прогона | P1 quality | воспроизвести FAIL `core/state`/`core/deploy` при `go test ./...` (workspace TEMP на OneDrive), проверить реальный TEMP/Defender; не маскировать flake retry-логикой |
| 4. Win11 VM acceptance | P0-06/P0-03/MVP | two-account deny-before-effects + same-user UAC; SMB shortcut/credentials, approved package install/detection/repeat/update/failure/cancel; real reboot Verify-only; затем Win10/LTSC |

Текущий пакет завершён тремя runtime commits (warnings) и остановлен после задачи 1 по решению владельца; точные проверки/команды и известный flake — HANDOFF. Без VM заканчивать независимые задачи 1–3, acceptance передать открытой. Новые features/Legacy не начинать. После документов и git status остановиться. Пользовательский untracked temp/windeploy-icon.png сохранять вне коммитов пакета.
