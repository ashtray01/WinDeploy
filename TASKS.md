# WinDeploy — controlled development

## Текущий этап

2026-11-07: аудит существующего preview, переход к Modern Windows 11/10 x64 (включая совместимые LTSC). Источник фактов — [docs/AUDIT.md](docs/AUDIT.md); команды и ограничения среды — [HANDOFF.md](HANDOFF.md). Наличие backend/страницы не означает завершение пользовательского сценария. Старые Stages 1–11 — история реализации, не критерий готовности MVP; полный прежний список сохранён в Git (`b990439:TASKS.md`).

## Завершено в этой итерации

- [x] Диагностика раннего выхода Modern: production build tag, отдельный console EXE и `scripts/start-modern-diagnostic.ps1`. Полная сборка/37 UI tests PASS вне sandbox; запуск окна остаётся FAIL (exit 1 до main, sandbox console показывает DLL/COM init panic). Это диагностическое улучшение, не исправленный GUI runtime.

- [x] Аудит всех подсистем, bridge и сценариев A–H, матрица доказательств и security findings (`5a52c7e`).
- [x] P0-01: detection error после exit 0 больше не даёт Success (`972e44e`); regression FAIL до изменения, PASS после.
- [x] Перепланирование P0–P3; Legacy вынесена в FROZEN.
- [x] Закрепить стратегию в AGENTS/архитектуре/README, обновить HANDOFF и итоговые результаты.

- [x] Runtime applicability подключён на общей границе Runner: свежий native/provider snapshot, проверка всего плана и GuardAdapter на start/resume; core/CLI/service tests и test/vet/build PASS. Отклонённый resume не отображается Completed.

- [x] Общий process-local gate package/deployment от подготовки до завершения; повторные start/resume/shutdown checks. Конкурентные barrier tests PASS, включая 20 повторов. Другие standalone mutations и отдельные процессы этим gate не покрыты.

## P0 — блокирующие ошибки

- [x] P0-02: применять требования ОС/архитектуры профиля на start/resume (core/deploy, CLI, service). Готово: несовместимая или неизвестная система не достигает изменяющего adapter; свежий inventory для resume; unit + CLI/service tests.
- [ ] P0-03: убрать ложный Success при отложенной reboot verification (engine/deploy/state/reporting/UI). Готово: зависимости ждут подтверждения, состояние переживает restart, resume не переустанавливает завершённое; persistence/negative tests и затем Win11 VM. Не лечить простым изменением флага.
- [ ] P0-04: честные результаты составных операций. Готово: отсутствие обязательного ярлыка/read-back или default printer не теряется в отчёте; warnings доступны; fake failure tests. Не смешивать подключённый UNC с выполненным shortcut requirement.
- [x] P0-05: единое исключение изменяющих операций и атомарный start/resume/shutdown. Готово: package page и deployment не запускают конкурирующие установки; два resume не пишут одну сессию; barrier-based concurrent service tests.
- [ ] P0-06: явная граница user/machine context. Готово: при Run as another admin пользовательские операции не направляются молча в чужие HKCU/Desktop/CredMan; fail-closed gate или проверенный elevated worker; two-account VM test.

## P1 — рабочий MVP

- [ ] Подтверждать target version после update, отличать unknown/absent/outdated; fail/cancel detection tests. Каталог и detection уже реализованы, переписывать их не требуется.
- [ ] Semantic profile preflight: category parameters, package refs, check_ref/diagnostic targets, dependency closure; ошибочный профиль отклоняется до первой мутации. Preview и execution используют одинаковые правила.
- [ ] Modern desktop smoke на доступной интерактивной Windows 11 с WebView2: настоящий HWND/DOM, native inventory comparison, действия/ошибки bridge. Отдельно от Vite build и happy-dom.
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

## Следующая итерация — 3 связанные задачи

| Задача | Приоритет / компоненты | Критерий готовности / проверка | Результат |
|---|---|---|---|
| 1. Runtime applicability | P0-02, deploy/CLI/service | tests start+resume с несовместимой/unknown OS, adapter calls=0; актуальная система при resume; go test/vet затронутых пакетов | Preview не противоречит safety gate выполнения |
| 2. Общий mutation gate | P0-05, Modern service | concurrent package/deploy/resume requests, один run/writer, shutdown race tests; service test/vet | Исключены конкурирующие установки и порча сессий |
| 3. Честная reboot boundary | P0-03, engine/deploy/reporting/UI | сначала согласовать локальный контракт; persistence/restart tests, зависимые не запускаются до verify; TS/UI tests | Непроверенная задача не выглядит успешной; явно указан предел VM verification |

Каждую задачу завершать отдельным коммитом; если третья требует отдельной итерации, закончить первые две и зафиксировать её незавершённой. Не добавлять новые features вместо устранения этих блокеров. После итерации — интеграционные checks затронутых путей и HANDOFF.
