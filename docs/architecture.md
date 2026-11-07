# Архитектура WinDeploy

## Стратегия и границы

Основной продукт — Modern для Windows 11/10 x64, включая совместимые LTSC; реальные builds/VM остаются отдельным доказательством. Windows 7/Legacy заморожена до решения владельца. Источник технических ограничений — [AUDIT.md](AUDIT.md), приоритетов — [../TASKS.md](../TASKS.md).

| Модуль | Ответственность |
|---|---|
| root `windeploy`, Go 1.20 | core, embedded catalog/profiles, CLI inspect/packages/run; без Wails |
| `desktop/modern`, Go 1.26.0 | Wails v3.1.0, service adapters/DTO, React/TypeScript, generated bindings; local replace root |
| `legacy`, Go 1.20/Walk | сохранённый нативный GUI; FROZEN |

Core остаётся на Go 1.20, Modern-specific зависимости изолируются. Ни директива go.mod, ни сборка не доказывают runtime совместимость. Старые Go 1.20.14/Win11 x86 результаты сохранены в истории; новые Win7 проверки не являются обязательными.

## Реальный поток приложения

React page → `frontend/src/bridge.ts` → generated Wails Service bindings → `desktop/modern/service` → core manager → Windows adapter. Transport реальный, production mock fallback отсутствует. Service содержит как read-only Inventory/preview, так и методы install/settings; старый запрет execute/install bridge больше не соответствует продукту.

Standalone pages используют managers напрямую. Выполнение профиля идёт через BuildPlan → `deploy.Runner` → `engine.Adapter` (`deploy.Adapter`) → те же core managers. CLI собирает зависимости отдельно. Именно различия между этими путями требуют integration tests: standalone Result с warnings сейчас не полностью переносится в TaskResult/отчёт.

## Компоненты

- `core/engine`: Task/TaskResult, DAG, precheck/execute/verify, bounded concurrency, opt-in retry, ResourceKey, checkpoints, resume. Не знает Windows и UI. Только Success/AlreadyInstalled удовлетворяют dependencies. MaxParallel/resources действуют внутри одного run.
- `core/profiles`: strict bounded YAML schema v1, inheritance, clone, выбранные компоненты и dependency closure. Проверяет структуру, но не все adapter-specific parameters. `catalog` и `profiles` содержат встроенные данные; CLI/Modern `--profiles` допускают внешние YAML.
- `core/inventory`: native system/CPU/RAM/disks/interfaces/registry apps, typed partial errors, collector и `Applicable/GuardAdapter`. deploy.Runner получает fresh System на каждом Start/Resume (DI или Native.System), проверяет весь план до первой adapter phase и подключает GuardAdapter. Сохранённый System/preview не разрешает execution.
- `core/packages`: source planning/detection, version comparison, HTTPS/file/WinGet, streaming hash/cache, Authenticode, silent execution и post-detection. Ошибка post-detection теперь Failed; target version, process-tree cancellation и reboot verification остаются ограничениями.
- `core/network`: read-only DNS/TCP/ICMP/HTTP/SMB-445, gateways/DHCP. Это не изменение IP/DNS конфигурации.
- `core/shares`: Credential Manager, WNet/1219, IShellLink; текущий пользователь процесса. `core/drivers`: SetupAPI/CM, INF catalog/signature, pnputil. `core/printers`: enumeration/PrintUI/default/GDI; TCP/IP port provisioning отсутствует.
- `core/browsers`: Chromium/Firefox policies и webapp shortcuts; `core/settings`: rename/timezone/Explorer/power, read-only update/licensing. Нет единого read-back/rollback контракта.
- `core/state`: отдельный snapshot FileStore; `core/deploy`: dispatch + полные session files + runner. `core/reporting`: JSON/escaped HTML без task parameters. Сессии содержат параметры и требуют trusted storage/ACL.
- `core/trust`: read-only проверка владельца/DACL файла и каталога перед мутирующим resume и запуском скачанного artifact. Разрешены записи только текущему пользователю, SYSTEM, Administrators и owner-mapped CREATOR OWNER; ссылки и reparse-точки каталога отклоняются. Это не аутентификация: администратор может сменить владельца.

## Контракты и известные нарушения

Engine проверяет DAG, не применимость Windows автоматически; Adapter обязан проверить requirements. Timeouts/отмена кооперативны; engine ждёт adapter, CommandContext не гарантирует завершение дерева installer children. Errors отдельных tasks отражаются в results; nil error Run не означает успех всех задач.

Success с reboot flag по контракту engine означает подтверждённую готовность. deploy.Adapter возвращает ErrRebootPending, который engine сохраняет как RebootRequired; этот status не разрешает dependencies. Runner сохраняет boot_id и per-task reboot_boot_ids, блокирует same/unknown boot и разрешает только Verify после смены boot identity. Проверка не повторяет Precheck/Execute; failure/cancel и checkpoint во время Verify сохраняют obligation. Pending session не Completed; report/CLI/UI показывают ожидание проверки. Исторический reboot Success без boot evidence автоматически не восстанавливается. Native reboot/fast startup VM acceptance пока открыта.

Windows boot query изолирован в inventory.Native.BootID: private NT class 90, kernel GUID, проверка size/error/nonzero, fail closed при недоступности. Layout сверяется с [первичным phnt header System Informer](https://github.com/winsiderss/systeminformer/blob/master/phnt/include/ntexapi.h); [Microsoft предупреждает об изменяемости NT query API](https://learn.microsoft.com/en-us/windows/win32/api/winternl/nf-winternl-ntquerysysteminformation). Это осознанная ограниченная зависимость с native read-only test на текущем Win11, не подтверждение смены GUID на всех Win10/LTSC/fast-startup режимах. Boot evidence не аутентифицирует владельца session.

Browser/windows Verify подтверждает эффект обратным чтением реестра/tzutil/powercfg; при недоступности чтения возвращается ошибка, а Execute сохраняет warning о неподтверждённом эффекте. Printer Verify дополнительно сверяет порт `IP_<адрес>`, а неподтверждённый default остаётся warning. Некоторые warnings исчезают при deploy dispatch. Session Completed означает завершение run, а не отсутствие Failed. Modern Service сериализует package/deployment preparation и execution общим operationGate, закрывая admission при shutdown. Gate не охватывает другие standalone mutations и другие процессы/CLI; межпроцессная защита session storage остаётся открытой.

User-scoped HKCU/Desktop/CredMan разрешаются в контексте процесса. Контролируемого elevated worker, сохраняющего исходного пользователя, нет. Перезапуск под другой учётной записью может направить настройки не тому пользователю; это P0 до применения на рабочих станциях.

Persistent files записываются через temp/Sync/Close/Rename. Это не гарантия power-loss durability; Windows ACL и один writer обязательны. Plan hash связывает snapshot с планом, но не аутентифицирует владельца. Отчёты исключают parameters, а журналы ограничены и преимущественно находятся в памяти; полный durable operation log ещё требуется.

## Проверка и развитие

Следующий пакет: user/machine context, честные составные результаты, Win11 VM acceptance. Applicability, process-local package/deployment gate и reboot software contract реализованы. Actual GUI EXE DOM/bridge smoke PASS с fresh WebView2 UDF; visible/default-user launch и reboot/install VM не подтверждены. Startup logger сохраняет stderr при отказе file log, --webview-data-dir явно выбирает отдельный профиль без очистки существующего. Затем расширение VM до Win10/LTSC. Legacy CI/packaging пока связаны с общим выпуском; Modern release gate запланирован, workflows не менялись.

Команды и результаты этой сессии — HANDOFF. Контракты engine — [execution.md](execution.md); аудит имеет приоритет над прежними заявлениями о готовности.
