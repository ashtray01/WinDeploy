# WinDeploy — Agent Handoff

## Состояние проекта
Этап 2 — Profiles. MULTI-TASK: восемь задач завершены (шесть ядра + модель профилей + YAML); далее inheritance и планы/примеры до лимита 10 задач.
Ветка: master. Коммиты: dfdadde preview, 0626c5e retries, de54b7a concurrency; snapshot storage — последний feat в git log.
Пользователь явно разрешил 5–10 последовательных задач и переход между этапами; это выше прежнего ограничения одной задачи. Работа выполняется одним агентом.

## Что уже реализовано
- Go-модуль windeploy, целевая версия ядра Go 1.20, без внешних зависимостей.
- Task/TaskResult/статусы; структурная Validate; OrderTasks с проверкой DAG и стабильным порядком.
- Run: последовательный Pending-only план, Adapter Precheck/Execute/Verify, блокировка зависимых задач, отмена и timeout.
- PreviewPlan: независимые метаданные без Parameters и фиктивных результатов; scope structure-and-dag-only.
- RunWithOptions: MaxAttempts, RetryDelay; повторы только RetryableError.Retryable()==true, новая попытка начинается с Precheck. Timeout включает все попытки/ожидания; TaskResult.Attempts считает попытки. Флаг перезагрузки сохраняется между попытками.
- MaxParallel по умолчанию 1, одинаковые Task.ResourceKey сериализуются; MSI-адаптеры обязаны назначать WindowsInstallerResource. Результаты в порядке плана, активные операции дожидаются завершения при отмене. Adapter должен быть thread-safe при MaxParallel>1.
- engine.Snapshot/NewSnapshot/Validate: версия, SHA-256 исходного плана, результаты без Parameters. Допускаются только известные безопасные diagnostics.
- state.FileStore: ограниченный JSON (4 MiB), запрет неизвестных полей/trailing data, проверка формата, temp+Sync+Close+Rename без обнуления старого файла. Каталог создаёт вызывающий код.
- Resume/ValidateForPlan: проверка точного плана и результатов до операций. Успешные результаты сохраняются, interrupted проходят Precheck, Failed требует RetryFailed. Attempts и reboot flag накапливаются. Обязательный reboot barrier пока отклоняется.
- RunOptions.Store: initial snapshot, Running до запуска и результаты после завершения. Save failure => ErrCheckpoint, отмена и ожидание активных операций. Save использует отдельный context с timeout 5s для сохранения при отмене. Тест реального FileStore restart с fake-установкой подтверждает отсутствие дублирования.
- profiles.Profile/Component schema v1, проверка категорий/ID/timeout/OS, tombstones Disabled. Запрещены распространённые secret keys и URL credentials; credential_ref ссылается на внешние credentials. Произвольные секреты в свободных значениях автоматически не распознаются.
- profiles.Load/Marshal: single-document YAML, 1 MiB, KnownFields, дубли/anchors/aliases/merge keys отклоняются, глубина/число узлов ограничены. Ошибки парсера не раскрывают входные значения. Зависимость go.yaml.in/yaml/v3 v3.0.4, upstream go 1.16; GOMODCACHE=cache/go-mod. go.mod остаётся go 1.20; tidy выполнен.

## Что проверено
Go 1.26.2 windows/amd64: go test ./core/engine/... ./core/state/... и go vet ./... — PASS; после задач профилей go test/vet ./core/profiles/... — PASS. Тесты каждой задачи запускались до её коммита; file replacement реально проверен на доступной Windows. C-компилятор не найден, race detector не запускался.
GOCACHE: `$env:GOCACHE = Join-Path $PWD 'cache\go-build'` (игнорируется Git). Стандартный cache ранее был недоступен.

## Что сейчас не работает
Нет реальных системных адаптеров/UI; inheritance и plans/examples профилей ещё не реализованы. Checkpoints/resume подключены, но live inventory завершённых задач не перепроверяется. Автозапуск после reboot/проверка смены boot ID не реализованы.
Go 1.20 и Windows VM-матрица, включая Windows 7 SP1 x86/x64, не проверены. GitHub CLI отсутствовал, remote/CI/releases не создавались.
Отмена требует кооперации адаптера; Run ждёт возврата и не прерывает его принудительно. Диагностика содержит только этап, исходные ошибки адаптера отбрасываются для защиты credentials.

## Незавершённая работа
Следом в текущей сессии: наследование, планы/примеры. Не выдавать их за реализованные.

## Архитектурные решения
- Task декларативен; методы в Adapter, не в сериализуемой модели. Adapter не меняет вложенные maps/slices.
- Precheck: Pending — выполнять; AlreadyInstalled/NotApplicable/Skipped — закончить. Только Success/AlreadyInstalled удовлетворяют зависимости.
- Success + RebootRequired означает, что Verify подтвердил готовность. Resume после обязательной перезагрузки ещё не реализован.
- Ошибки задач находятся в results; error Run — неверный ввод или ctx.Err(). Pending-only защищает от случайного переисполнения.
- OrderTasks O(V²+E), при равенстве выбирает наименьший исходный индекс. Выход — поверхностная копия Task. Preview копирует вложенные метаданные.
- Retries включаются явно. Адаптер обязан гарантировать безопасность RetryableError; обычные ошибки не повторяются.
- Snapshot не аутентифицирован и предназначен для доверенного локального каталога. Path принадлежит одному процессу; mutex FileStore защищает только экземпляр. Windows ACL задаёт администратор, 0600 не заменяет ACL. Не заявлять гарантии atomicity/durability при сбое питания: Rename зависит от файловой системы. Хеш связывает точный план, но не является защитой от подмены.

## Следующая задача
Реализовать single-parent inheritance, затем выбранные компоненты/планы/clone/examples; остановиться после десятой задачи.

## Инструкция следующему агенту
Прочитать AGENTS/TASKS, core/engine/task.go, plan.go, executor.go и тесты. Проверить status/log.
Прочитать раздел 9 исходного ТЗ. Профили должны описывать компоненты, не выполнять операции и не содержать credentials. Schema version, строгая валидация, single-parent inheritance с циклами, преобразование в Pending Task-план и PreviewPlan. Для YAML использовать поддерживаемую YAML-org библиотеку go.yaml.in/yaml/v3 v3.0.4 (go.mod upstream: go 1.16; https://github.com/yaml/go-yaml/tree/v3.0.4), не писать собственный YAML parser.
Каждую задачу сохранять отдельным коммитом с тестами; HANDOFF обновлять после 2–3 задач.

## Чего нельзя делать
Не устанавливать Wails, не запускать установщики и административные операции на хосте. Не заявлять Windows 7 совместимость без VM.
Для записи .git ранее требовался escalated exec и `git -c safe.directory=C:/Users/ashtr/OneDrive/Desktop/git/WinDeploy ...`. Не менять глобальную конфигурацию и identity. Не коммитить binaries/runtime state/credentials.
