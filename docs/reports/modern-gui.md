# Modern GUI — итог пакета из 10 задач

Ветка `master`, база `4b6250c`, 2026-11-06. Получен первый работающий Windows desktop WinDeploy: тёмный React/TypeScript UI внутри настоящего Wails/WebView2 EXE. Реальные Inventory, профили и журнал подключены к общему ядру. Системные изменения через GUI отсутствуют. Релиз не публиковался.

## Выполненные задачи и коммиты

| № | Результат | Коммит |
|---|---|---|
| 1 | Изолированный Go 1.20 test script; отдельный проверенный toolchain, amd64/386 | `f1b893f`, `112dc64` |
| 2 | Отдельный Modern Go-модуль, стабильный Wails v3.1.0, root go.mod не повышен | `e853495` |
| 3 | Wails + React + TypeScript, Windows GUI EXE и настоящий запуск | `3243666` |
| 4 | Тёмные графитовые поверхности, синие акценты, UI primitives и responsive CSS | `ad7ab1e` |
| 5 | Sidebar, topbar/search, content, statusbar; будущие модули честно disabled | `dbb6c86` |
| 6 | Generated typed bridge, настоящий Collector, async/cancel/partial/errors | `96163d5` |
| 7 | Реальный Dashboard и повторное сканирование | `3dc6d2f` |
| 8 | Inventory detail tables: поиск, сортировка, пагинация, неизвестные значения | `42569e1` |
| 9 | Существующие YAML, наследование, выбор, core Preview, DAG, applicability | `5048915` |
| 10 | Настоящий журнал, ошибки/recovery, сборка и полный desktop smoke | `169c239` |

Промежуточные HANDOFF checkpoint commits: `c812679`, `fb7c5ad`. Итоговая документация сохраняется следующим отдельным docs-коммитом.

## Проверки

- Отдельный официальный Go 1.20.14: root test/vet/build для Windows amd64 **и** 386 — PASS. Повторные `go test -count=1 ./...` обеих архитектур — PASS; Windows Inventory tests исполнились на настоящей текущей Windows. SHA-256 toolchain проверен перед распаковкой. Системный Go/PATH не заменялся.
- Root Go 1.26.2: tests/vet/build — PASS; `go mod verify` — all modules verified. Root go.mod/go.sum сохранены без добавления Wails.
- Modern: Go tests, отдельный повтор с `-count=1`, vet, EXE build — PASS. Cache verification через временный workspace — all modules verified; причина workspace описана в [сборке](../modern.md).
- UI: 8 test files / **14 tests PASS**; TypeScript strict check и Vite production build PASS. Проверены недоступные controls, unknown progress, navigation, asynchronous bridge, partial/empty values, таблицы, профильный выбор/DAG/ошибка, реальный журнал и восстановление после render error. Test fixtures находятся только в тестах.
- Настоящий Windows 11 x64 build 26200 / WebView2 156.0.4314.58: EXE создаёт окно WinDeploy и загружает production assets. Playwright подключён к его WebView2 через opt-in localhost CDP, без dev server/mock backend/скачивания отдельного браузера.
- Desktop smoke окончательного EXE PASS: hostname и 200 visible app entries совпали с отдельно собранной native CLI в том же пользовательском контексте. Повторный scan увеличил generation и завершился. Проверены страницы программ/сети/томов/системы, поиск и сортировка; Administrator показал настоящее ребро DAG; Legacy показал несовместимость с текущей ОС; журнал показал фактический preview event. Нет execution controls; все пять будущих модулей disabled. Viewports 1366×768 и 1024×640 без горизонтального переполнения оболочки, **0 page errors**.
- Screenshots проверены визуально: `local/modern-dashboard-final.png`, `local/modern-profiles.png`. Они содержат сведения компьютера, поэтому остаются только локально и не коммитятся. Binaries, module/npm caches, runtime logs также исключены из Git.
- `git diff --check` и gofmt для изменённых Go-файлов — PASS.

## Ограничения и следующий шаг

Это первый read-only Modern GUI, не готовый установщик. OS compatibility в Preview не доказывает доступность установщиков, права, исправность программ или возможность выполнения операции. Parameters не экспортируются в UI. Журнал хранится только в памяти процесса; preview/session не сохраняются.

Go 1.20 на текущей Windows не доказывает runtime Windows 7. Полная VM-матрица Windows 10/11 и Legacy 7 SP1, разные DPI/мониторы, отсутствие/старый WebView2 и ARM не проверены. Race detector не запускался: доступного C toolchain нет. Поведение после фатальной ошибки среды имеет нативное сообщение, но отсутствующий Runtime на хосте намеренно не воспроизводился удалением зависимости.

Sandbox-пользователь имеет другой HKCU: его CLI показала 167 записей против 200 интерактивного пользователя. Это различие контекстов, а не замена данных; корректный smoke запускает обе программы под одним пользователем. GUI запускается в интерактивной сессии с обычным доступом к пользовательскому WebView2 cache. Обычный запуск не включает CDP/debug tools.

Следующая задача: ограниченный Modern hardening — проверить запуск из чистого checkout и сценарий отсутствующего WebView2 в Windows VM, затем DPI/keyboard accessibility. Не начинать Package Manager, драйверы или системные настройки. Конкретные команды и порядок — [HANDOFF](../../HANDOFF.md).
