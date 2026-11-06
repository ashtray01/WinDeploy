# WinDeploy — Agent Handoff

## Состояние проекта
Ветка master. Завершены все 10 задач текущего Modern GUI пакета; следующий этап Package Manager не начат и не разрешён текущим объёмом. Последняя реализация: 169c239. База сессии: 4b6250c.
Подробный итог: docs/reports/modern-gui.md; сборка/запуск/контракты: docs/modern.md. TASKS.md отмечает Modern 1–10, весь будущий Stage 4 не объявлен завершённым.

## Что реализовано
- Изолированный desktop/modern: Wails/CLI/runtime v3.1.0 stable, Go 1.26.0+; проверен системный Go 1.26.2. Root windeploy сохраняет Go 1.20 и прежние зависимости. Root ./... не включает Modern.
- Настоящий Windows EXE bin/WinDeploy-Windows-x64.exe: Wails + React/TypeScript, тёмные поверхности/синие акценты, sidebar/topbar/statusbar, реальные Dashboard и Inventory tables, поиск/сортировка/пагинация, 1366×768 и минимальное окно 1024×640. Будущие модули disabled/«Скоро».
- service использует inventory.Native/Collect асинхронно: один scan, deadline 30s, cooperative cancel, partial/errors/unknown. Reports после публикации immutable, журнал ограничен 200 настоящими событиями. ServiceShutdown отменяет и дожидается scan. Typed bindings генерируются Wails -noevents; production fallback/fixtures отсутствуют.
- Существующие 8 YAML встроены через profiles/embed.go (Go1.20 embed); --profiles path читает внешние YAML. Core strict parser/Resolve/Preview/Applicable; выбор включает dependency closure; DAG и incompatible/blocked/unknown честные. Parameters не экспортируются, execution API не подключён. Неудачный reload сбрасывает старый registry.
- Реальный журнал текущего процесса, безопасные сообщения ошибок среды, UI ErrorBoundary и native dialog при fatal startup. Журнал не сохраняется на диск. WebView2 автоматически не устанавливается.
- Core engine/state/profiles/inventory и read-only CLI предыдущей сессии сохранены. Их контракты/ограничения описаны в docs/architecture.md, execution.md, profiles.md, inventory.md и docs/reports/autonomous-inventory.md.

## Проверки
- Официальный отдельный Go 1.20.14, SHA-256 проверен до запуска. scripts/test-core.ps1: test/vet/build root windows/amd64 и windows/386 PASS. Повторные go test -count=1 ./... обеих архитектур PASS. Native Inventory tests реально исполнялись на текущей Windows, не Win7 VM.
- Root Go1.26.2 test/vet/build PASS; go mod verify PASS; исходные root go.mod/go.sum не повышены ради GUI.
- Modern Go test (также -count=1)/vet/build PASS, module cache verify PASS через временный workspace (см. docs/modern.md).
- UI 8 файлов / 14 tests PASS, strict TypeScript/Vite production build PASS.
- Финальный настоящий EXE на Windows11 x64 build26200/WebView2 156.0.4314.58: HWND/DOM/assets, native CLI comparison hostname+apps, rescan generation, Inventory tables/search/sort, Administrator DAG, Legacy incompatibility, real journal PASS; 1366×768 и 1024×640, 0 page errors. Скрипт scripts/gui-modern-smoke.mjs; это реальный desktop, не Vite browser preview.
- Private screenshots local/modern-dashboard-final.png и local/modern-profiles.png визуально проверены; binaries/logs/private data не в Git. Diff --check и gofmt PASS.
- После всех 10 задач выполнена проверка интеграции и регрессий; правило AGENTS whole-project check соблюдено.

## Ограничения
- Windows7 SP1 VM не проверена. Успешный Go1.20 на текущей Windows не доказывает Win7 runtime. Modern не предназначен для Win7; Legacy GUI отсутствует.
- Не проверены Windows10 VM, missing/old WebView2, multi-monitor/DPI и ARM. Race detector не запускался: C compiler недоступен.
- Нет Package Manager/реальных изменяющих adapters/драйверов/изменения сети. Preview — структура/DAG/OS requirements, не проверка установщиков/источников/прав/installed state.
- Core native API синхронны, отмена кооперативная; Apps не включает AppX/MSIX/portable, HKCU зависит от пользователя процесса. Больше границ Inventory в docs/inventory.md.
- Runtime cache %LOCALAPPDATA%/WinDeploy/WebView2; не запускать несколько экземпляров с одним cache. Журнал в памяти, параметры YAML скрыты, GUI editing/persistent sessions отсутствуют.
- GitHub remote/CI/releases не создавались. Git status должен быть чистым после финального docs-коммита.

## Коммиты сессии
1 f1b893f — core Go1.20 verification script.
2 e853495 — isolated pinned Wails module.
3 3243666 — real desktop bootstrap.
Checkpoint c812679.
4 ad7ab1e — design system.
112dc64 — actual Go1.20 amd64/386 verification after download.
5 dbb6c86 — navigation shell.
6 96163d5 — typed native async bridge + checkpoint.
7 3dc6d2f — real dashboard/native smoke.
8 42569e1 — Inventory detail tables.
9 5048915 — Profiles/DAG/applicability.
Checkpoint fb7c5ad.
10 169c239 — journal/errors/full integration/build script.
Итоговая документация — следующий docs-коммит, посмотреть git log.

## Архитектурные решения
Не переносить Wails в root module. Core остаётся Go1.20; Modern зависимости обновляются отдельно. Generated bindings, одна production transport boundary, UI без Windows API и execution logic. Report snapshot/preview read-only. Go mod verify Modern использует local/_modern-verify/go.work только во время проверки: Go1.26.2 иначе пытается проверить zip локального replacement. Это не root workspace/toolchain upgrade.

## Следующая конкретная задача
Проверить воспроизводимую сборку Modern из чистого checkout и missing-WebView2 startup в изолированной Windows VM, записать результаты. Затем DPI/keyboard accessibility; Package Manager не начинать.

## Инструкция следующему агенту
1. git status/log; AGENTS/HANDOFF/TASKS, затем docs/modern.md. Не повторять исследование готового ядра.
2. Build: scripts/build-modern.ps1 (-InstallDependencies для clean checkout). CLI закреплён cache/tools/wails-3.1/wails3.exe; npm/Go caches внутри cache/. Toolchain Go1.20.14: local/_toolchains/go1.20.14/go/bin/go.exe, cache/go-build-go120. Подчёркивания исключают toolchain/module sources из root ./....
3. Core: scripts/test-core.ps1 -Go (Join-Path $PWD 'local/_toolchains/go1.20.14/go/bin/go.exe') -BuildCache go-build-go120. Не выдавать это за Win7 VM test.
4. GUI smoke: независимая native CLI bin/WinDeploy-Inspect-Windows-x64.exe; GUI --smoke-port 9223; node scripts/gui-modern-smoke.mjs. Оба процесса под одним интерактивным пользователем: sandbox HKCU показал167, настоящий пользователь200 entries; сравнение разных HKCU неверно. CDP включается только флагом. Для background GUI в tool session держать PowerShell WaitForExit, иначе tool host может завершить child; запускать smoke сразу после Start-Process. Обычный запуск без smoke-port.
5. GUI требует обычного доступа к user cache вне файловой sandbox; interactive launch и same-user smoke выполнялись через escalated exec. Это не разрешение системных изменений. Для пересборки закрывать только собственный EXE с проверкой Process.Path; не трогать чужие WebView/browser processes.
6. Git .git защищён sandbox: commit через escalated exec с git -c safe.directory=C:/Users/ashtr/OneDrive/Desktop/git/WinDeploy, без global config/identity. После atomic task tests/commit/TASKS; HANDOFF каждые2–3; whole-project после10–15.

## Чего нельзя делать
Не устанавливать пакеты/драйверы, не менять сеть/защиту/Windows settings на рабочем хосте; изменяющие операции только отдельно авторизованной VM. Не добавлять fixture data/Success в production GUI. Не коммитить EXE, кэши, runtime logs, hostname/IP/MAC/app lists и credentials. Не публиковать release как production-ready по одному desktop smoke.
