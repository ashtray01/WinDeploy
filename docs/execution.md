# Выполнение и восстановление

`Run(ctx, tasks, adapter)` сохраняет прежнее поведение: Pending-only план, одна задача и одна попытка одновременно. `RunWithOptions` принимает MaxAttempts, RetryDelay, MaxParallel и Store. Нули означают одну попытку/одну задачу/нет дополнительной задержки и нет хранилища.

Retry выполняется только для ошибки, реализующей RetryableError с Retryable()==true, включая обёрнутую ошибку. Адаптер обязан гарантировать безопасность повтора. Каждая попытка начинается с Precheck, чтобы обнаружить уже выполненные побочные эффекты. Общий task timeout включает все попытки и ожидания. RebootRequired сохраняется между попытками даже при ошибке.

При MaxParallel>1 Adapter должен быть thread-safe. Зависимые задачи ждут Success/AlreadyInstalled; остальные исходы блокируют зависимых, но независимые продолжаются. Запуск готовых задач следует порядку OrderTasks, фактический порядок завершения зависит от адаптера. Выходные results всегда в порядке плана. Task.ResourceKey сериализует все фазы и ожидания задачи. MSI-адаптер обязан выставить engine.WindowsInstallerResource для каждой операции Windows Installer — сам движок не определяет тип EXE/MSI по параметрам.

Snapshot хранит версию, SHA-256 точного Pending-плана (включая параметры) и результаты. В него не попадают сами Parameters, произвольные diagnostics и команды. Snapshot.Validate проверяет формат, ValidateForPlan — соответствие исходному плану, ID и запущенные зависимости. Данные не аутентифицированы: храните их в доверенном локальном каталоге с Windows ACL и одним процессом-владельцем. Запись в FileStore защищена mutex только внутри экземпляра.

FileStore сохраняет temp-файл в том же каталоге, выполняет Sync/Close и Rename. Ошибки до замены не обнуляют старый checkpoint; временный файл убирается. JSON ограничен 4 MiB, неизвестные поля и дополнительные документы запрещены. Гарантии atomicity/durability при power loss на Windows не заявляются: они зависят от filesystem. Каталог должен заранее создать вызывающий код; mode 0600 не заменяет Windows ACL.

RunOptions.Store сохраняет состояние до первой операции, Running перед запуском задачи и финальный результат после неё. Ошибка Save возвращает ErrCheckpoint, отменяет активные операции, запрещает новые и дожидается очистки адаптеров. Для сохранения при отмене используется отдельный context с deadline 5 секунд; Store обязан его соблюдать. Неисправный диск может помешать сохранить последние исходы: после восстановления interrupted-задачи проверяются Precheck. Между фазами/отдельными retry attempts checkpoint не делается, поэтому после внезапного завершения счётчик попыток может быть неполным.

Пример восстановления (оригинальный Pending-план должен быть загружен отдельно):

```go
store := &state.FileStore{Path: sessionPath}
snapshot, err := store.Load(ctx)
// Обработать err, не заменять повреждённое состояние пустым автоматически.
results, err := engine.Resume(ctx, tasks, adapter, snapshot, engine.ResumeOptions{
    Execution: engine.RunOptions{Store: store, MaxParallel: 2},
})
// Обработать err и статусы каждого result, даже если err == nil.
```

Resume не запускает повторно Success/AlreadyInstalled/NotApplicable и явно пропущенные задачи. Failed сохраняется, если администратор явно не задал RetryFailed. Running/Cancelled и пропуски из-за зависимостей возвращаются в план через Precheck. Успешные сохранённые результаты не перепроверяются по live inventory; это доверенный отчёт предыдущего выполнения, не факт текущего состояния системы.

RebootRequired по контракту engine — флаг проверенного успеха. Текущий deploy.Adapter нарушает этот контракт: пропускает Verify для reboot, после чего Success сохраняется Resume без новой проверки. Отложенная post-reboot verification не реализована; зависимые задачи могут стартовать преждевременно (P0-03, docs/AUDIT.md). Resume отклоняет status RebootRequired; boot-ID validation и автоматический запуск при входе отсутствуют. Не обещать корректное reboot continuation.

## Интеграция профилей: `core/deploy`

`deploy.Adapter` реализует `engine.Adapter` и маршрутизирует компоненты профиля по `Category`:

| Category | Precheck | Execute | Verify |
| --- | --- | --- | --- |
| packages | план каталога: up-to-date → AlreadyInstalled, incompatible → NotApplicable | `packages.Manager.Install` по `package_ref` | повторная детекция |
| shares | перечисление WNet: UNC уже подключён → AlreadyInstalled | `shares.Manager.Add` (пароль только из Credential Manager) | подключение подтверждено |
| drivers | устройство с Hardware ID уже ok → AlreadyInstalled | `drivers.Manager.Install` | устройство ok |
| printers | очередь с именем существует → AlreadyInstalled | `printers.Manager.Add` | очередь существует |
| browsers | Pending | политики Chromium/Firefox | nil без read-back; warnings теряются в deploy |
| windows | Pending | computer-name/timezone/explorer/power-plan | nil без read-back |
| diagnostics | Pending | DNS/DHCP/gateway/TCP/ICMP/HTTP/SMB | Execute |

Пароли никогда не читаются из параметров профиля; для shares используется Windows Credential Manager. Параметры компонентов в отчёт не попадают.

## Сессии и отчёты

`deploy.Store` хранит `sessions/<id>.json` (0600, атомарная запись) с исходным Pending-планом, snapshot и итогом. `deploy.Runner.Start` создаёт сессию до первой операции и пишет полную сессию на каждый checkpoint engine. `Resume` загружает план и snapshot, сохраняет успешные результаты и повторяет только разрешённые задачи; `retryFailed` требуется явно. Сессия не содержит паролей, но содержит параметры компонентов, поэтому каталог должен быть приватным.

После завершения/прерывания `reporting.Build` создаёт JSON и автономный HTML (`reports/windeploy-<session>-<time>.html`): профиль, система, итог по статусам, таблица задач с временем/попытками/ошибками, флаг перезагрузки. HTML экранирует все значения; параметры компонентов не включаются. Повторная запись в тот же файл запрещена.

CLI `windeploy-run`: `plan`, `run --yes`, `resume --session ID [--retry-failed] --yes`, `sessions`, `report`. Modern GUI: страница «Выполнение» — выбор профиля и компонентов, подтверждение запуска, живой прогресс через опрос сессии, отмена, список сессий, просмотр и продолжение, пути отчётов.

Проверено на текущей Windows: реальная сессия `selftest` с diagnostics-компонентом выполнена через CLI и через service-тест; созданы session JSON, HTML/JSON отчёты; повторный запуск после отмены продолжает задачи; завершённая сессия без `retryFailed` не перезапускается. Изменяющие компоненты (packages/drivers/printers/shares/browsers/windows) в этой сессии не выполнялись — нужна VM с правами администратора.
