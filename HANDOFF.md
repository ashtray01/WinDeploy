# WinDeploy — Agent Handoff

## Состояние проекта
Этап 1 — Core Foundation. MULTI-TASK: dry-run и retries завершены; выполняется управление параллельностью.
Ветка: master. Коммиты текущей сессии — см. git log (preview: dfdadde, retries: последний feat).
Пользователь явно разрешил 5–10 последовательных задач и переход между этапами; это выше прежнего ограничения одной задачи. Работа выполняется одним агентом.

## Что уже реализовано
- Go-модуль windeploy, целевая версия ядра Go 1.20, без внешних зависимостей.
- Task/TaskResult/статусы; структурная Validate; OrderTasks с проверкой DAG и стабильным порядком.
- Run: последовательный Pending-only план, Adapter Precheck/Execute/Verify, блокировка зависимых задач, отмена и timeout.
- PreviewPlan: независимые метаданные без Parameters и фиктивных результатов; scope structure-and-dag-only.
- RunWithOptions: MaxAttempts, RetryDelay; повторы только RetryableError.Retryable()==true, новая попытка начинается с Precheck. Timeout включает все попытки/ожидания; TaskResult.Attempts считает попытки. Флаг перезагрузки сохраняется между попытками.

## Что проверено
Go 1.26.2 windows/amd64: go test ./core/engine/... и go vet ./... — PASS после каждой из двух задач.
GOCACHE: `$env:GOCACHE = Join-Path $PWD 'cache\go-build'` (игнорируется Git). Стандартный cache ранее был недоступен.

## Что сейчас не работает
Нет реальных системных адаптеров/UI. Нет параллельности, persistent state/resume/checkpoints (следующие задачи этой сессии).
Go 1.20 и Windows VM-матрица, включая Windows 7 SP1 x86/x64, не проверены. GitHub CLI отсутствовал, remote/CI/releases не создавались.
Отмена требует кооперации адаптера; Run ждёт возврата и не прерывает его принудительно. Диагностика содержит только этап, исходные ошибки адаптера отбрасываются для защиты credentials.

## Незавершённая работа
Следом в текущей сессии: bounded concurrency + ключи ресурсов; file snapshots; resume; checkpoint integration. Не выдавать их за реализованные.

## Архитектурные решения
- Task декларативен; методы в Adapter, не в сериализуемой модели. Adapter не меняет вложенные maps/slices.
- Precheck: Pending — выполнять; AlreadyInstalled/NotApplicable/Skipped — закончить. Только Success/AlreadyInstalled удовлетворяют зависимости.
- Success + RebootRequired означает, что Verify подтвердил готовность. Resume после обязательной перезагрузки ещё не реализован.
- Ошибки задач находятся в results; error Run — неверный ввод или ctx.Err(). Pending-only защищает от случайного переисполнения.
- OrderTasks O(V²+E), при равенстве выбирает наименьший исходный индекс. Выход — поверхностная копия Task. Preview копирует вложенные метаданные.
- Retries включаются явно. Адаптер обязан гарантировать безопасность RetryableError; обычные ошибки не повторяются.

## Следующая задача
Добавить bounded concurrency и общий ключ ресурса для взаимного исключения конфликтующих задач (MSI — общий windows-installer).

## Инструкция следующему агенту
Прочитать AGENTS/TASKS, core/engine/task.go, plan.go, executor.go и тесты. Проверить status/log.
Сохранить Run последовательным по умолчанию. RunWithOptions.MaxParallel ограничивает число одновременно выполняющихся задач; незавершённые зависимости не запускать. Результаты вернуть в детерминированном порядке плана. Не оставлять фоновые операции после отмены. Проверить реальные перекрытия, ограничения параллельности, ресурсные конфликты, зависимости/ошибки/отмену через каналы fake-адаптеров.
После этой задачи реализовать задачи file snapshots, resume и checkpoints из TASKS, отдельными коммитами и тестами. HANDOFF обновлять после 2–3 задач.

## Чего нельзя делать
Не устанавливать Wails, не запускать установщики и административные операции на хосте. Не заявлять Windows 7 совместимость без VM.
Для записи .git ранее требовался escalated exec и `git -c safe.directory=C:/Users/ashtr/OneDrive/Desktop/git/WinDeploy ...`. Не менять глобальную конфигурацию и identity. Не коммитить binaries/runtime state/credentials.
