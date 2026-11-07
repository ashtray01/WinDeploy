# WinDeploy — controlled development

## Текущий этап

2026-11-07: Modern startup stderr/UDF recovery и программный контракт reboot boundary реализованы. Реальный DOM/bridge smoke GUI EXE с новым UDF PASS на текущем Win11; visible/default-UDF launch и native reboot VM ещё открыты. Источники фактов — docs/AUDIT.md и HANDOFF.md.

## Завершено в этой итерации

- [x] Пользователь подтвердил запуск. Launcher исправляет UTF-8, сохраняет startup log в явном workspace каталоге (--log-dir), ACL/старый AppData log не меняет. Реальный launcher/DOM/bridge smoke PASS, UTF-8 русский текст подтверждён, Access denied в новом логе нет. Smoke ждёт асинхронное появление страницы CDP.

- [x] Startup stderr fallback, file-open failure tests, WebView2 write probe/explicit UDF без очистки старого профиля, launcher arguments; actual GUI EXE DOM/bridge smoke PASS с новым UDF (`b1b6dac`). Native Inventory совпал, pageErrors=0; visible/default-user acceptance отдельно.
- [x] P0-03 software contract: ErrRebootPending → RebootRequired, blocked dependencies, persisted per-task boot evidence, same/unknown-boot gate, Verify-only recovery без повторного Execute, pending CLI/report/UI (`d4c4171`). Root/Modern checks PASS, UI 38/38. Реальный reboot/install VM остаётся открытым.

Ранее завершены исходный аудит/roadmap (`5a52c7e`), package detection fail-closed (`972e44e`), runtime applicability (`d81df3e`), process-local package/deployment gate (`2d3562c`). Старые test результаты — история HANDOFF/Git, не новая VM evidence.

## P0 — блокирующие ошибки

- [ ] Modern startup acceptance: DOM/bridge настоящего GUI EXE с явным fresh UDF PASS; вручную подтвердить visible/default-UDF запуск владельцем. User script-paste ошибка исправляется запуском файла .ps1, старый UDF давал 800700aa; данные/ACL не сбрасывались.

- [x] P0-02: применять требования ОС/архитектуры профиля на start/resume (core/deploy, CLI, service). Готово: несовместимая или неизвестная система не достигает изменяющего adapter; свежий inventory для resume; unit + CLI/service tests.
- [ ] P0-03 VM acceptance: программный контракт исправлен (`d4c4171`), pending не Success/Completed; Verify-only после смены kernel BootID. Unit/file/CLI/service/UI tests PASS. Осталось: реальный reboot/fast startup + native readiness/dependencies/report в Win11 VM, затем Win10/LTSC.
- [ ] P0-04: честные результаты составных операций. Готово: отсутствие обязательного ярлыка/read-back или default printer не теряется в отчёте; warnings доступны; fake failure tests. Не смешивать подключённый UNC с выполненным shortcut requirement.
- [x] P0-05: единое исключение изменяющих операций и атомарный start/resume/shutdown. Готово: package page и deployment не запускают конкурирующие установки; два resume не пишут одну сессию; barrier-based concurrent service tests.
- [ ] P0-06: явная граница user/machine context. Готово: при Run as another admin пользовательские операции не направляются молча в чужие HKCU/Desktop/CredMan; fail-closed gate или проверенный elevated worker; two-account VM test.

## P1 — рабочий MVP

- [ ] Подтверждать target version после update, отличать unknown/absent/outdated; fail/cancel detection tests. Каталог и detection уже реализованы, переписывать их не требуется.
- [ ] Semantic profile preflight: category parameters, package refs, check_ref/diagnostic targets, dependency closure; ошибочный профиль отклоняется до первой мутации. Preview и execution используют одинаковые правила.
- [ ] Modern desktop acceptance: actual EXE DOM/bridge/native Inventory smoke PASS с explicit UDF; проверка запускалась Hidden, MainWindowHandle=0. Нужна visible/default-user проверка, Windows 10/LTSC отдельно.
- [ ] Win11 VM: один одобренный пакет (7-Zip), clean install → detection → повтор без установки → update/failure/cancel; UI progress, exit codes и отчёт совпадают с системой. Затем остальные пакеты каталога и Windows 10/LTSC.
- [ ] SMB VM/server: UNC access, credential storage в правильном контексте, conflict preview/consent, shortcut existence, повтор и ошибки. Native WNet/list/link код уже есть; полный сценарий не подтверждён.
- [ ] Persistent безопасный журнал с error codes и связью task/session; не включать secrets/raw installer output. UI сообщает об ошибках polling; отчёт сохраняет warnings/partial outcomes.
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

## Следующая итерация — 3 задачи

| Задача | Приоритет / компоненты | Критерий готовности / проверка |
|---|---|---|
| 1. User/machine context | P0-06, managers/service | fail-closed gate или worker с исходным SID; user-scoped mutations не направляются другому admin; unit/service tests, two-account VM отдельно |
| 2. Честные составные результаты | P0-04, deploy/shares/browsers/report | обязательный UNC+shortcut/webapp effect не теряется, warnings доступны; negative tests; VM отдельно |
| 3. Win11 VM acceptance | startup/P0-03/основной package scenario | visible/default-UDF launch, 7-Zip install/detection/repeat; настоящая reboot boundary, boot GUID changed, Verify-only/dependencies/report; fast startup; затем отдельная Win10/LTSC матрица |

Текущий пакет: startup diagnostics/recovery (`b1b6dac`) и persisted reboot verification (`d4c4171`). Root test/vet/build, полная Modern сборка PASS; UI 38/38; actual GUI EXE DOM/bridge smoke PASS с новым UDF. Native reboot/install и visible HWND не подтверждены. Без VM продолжать независимые задачи 1–2, VM acceptance оставить открытой. Подробности/точные команды — HANDOFF.

Каждую задачу завершать отдельным commit; после пакета документы, clean Git и stop. Новые features и Legacy не начинать.
