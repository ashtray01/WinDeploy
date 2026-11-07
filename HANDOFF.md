# WinDeploy — HANDOFF

## Текущий этап и факты

2026-11-07: первая итерация TECHNICAL AUDIT & CONTROLLED DEVELOPMENT завершена. Основной продукт — Modern Windows 11/10 x64 (совместимые LTSC требуют проверки). Windows 7/Legacy — FROZEN до отдельного решения владельца. Root go.mod остаётся 1.20, Modern — отдельный Wails v3.1.0/Go 1.26 модуль. Версия упаковки 0.1.0-preview.1, не production. Полная техническая картина: docs/AUDIT.md, не старые галочки Stages 1–11.

## Завершённые задачи

1. Аудит компонентов core/CLI/Modern/bridge/config/CI, сценариев A–H и безопасности; evidence matrix, конкретные функции/дефекты и roadmap.
2. Перепланирование P0–P3 и отдельный FROZEN / DEFERRED — Windows 7; подготовлены три задачи следующей итерации с критериями.
3. Обновлены AGENTS.md, архитектура, README, execution/release docs и этот HANDOFF: устаревшие обещания read-only bridge и reboot continuation устранены.
4. Небольшой P0-01 исправлен: Manager.runItem больше не сохраняет Success, когда post-install Detect вернул ошибку. Регрессионный тест показал FAIL до изменения, PASS после. Установщики не запускались.

## Коммиты

- База аудита `b990439`, ветка `master`, исходное дерево чистое.
- `972e44e` — fix(packages): fail verification when installed state is unknown.
- `5a52c7e` — docs(audit): assess runtime gaps and Modern MVP risks.
- `fbbc7f1` — docs(roadmap): prioritize Modern MVP and freeze Windows 7.
- Завершающий документационный коммит с актуализированными правилами/архитектурой/HANDOFF следует за этими тремя; его точный hash — `git log -1 --oneline`. Самоссылка на собственный hash в коммите не используется.

## Фактически выполненные проверки

Установленный Go: go1.26.2 windows/amd64. Новые зависимости и старые toolchains не устанавливались.

- Root: `go test ./... -count=1 -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip'`, `go vet ./...`, `go build ./...` — PASS. Две системные записывающие проверки явно исключены; native read-only tests и temp-file tests выполнялись.
- P0: `go test ./core/packages -run TestInstallDetectionFailureCannotReportSuccess -count=1` — FAIL до фикса; `go test ./core/packages ./core/deploy ./cmd/windeploy-packages -count=1` и vet этих пакетов — PASS после. gofmt выполнен.
- Финальный root с `-cover -count=1` и теми же двумя исключениями — PASS вне sandbox с workspace TEMP. Sandbox попытка падала при native COM/KnownFolder DLL initialization. Engine 96.6%, profiles 93.8%, inventory 83.6%, network 82.2%, packages 63.6%, deploy 43.0%; остальные проценты в AUDIT. Не выдавать coverage за E2E.
- Frontend `npm test`: 37/37, 16 файлов; `npm run build`: strict tsc + Vite PASS. Lint script/config отсутствует — lint не выполнялся.
- Полный `scripts/build-modern.ps1`: PASS — regenerated bindings, UI tests/build, Modern go test/vet, module verification, `bin/WinDeploy-Windows-x64.exe`. Сначала sandbox Wails падал user32 init; следующий запуск — системный TEMP Access denied. Разрешённый запуск вне sandbox с TEMP/TMP/GOTMPDIR внутри repo прошёл. Прямой ранний Modern service test падал на checkpoint в системном TEMP; итоговый полный build прошёл с workspace TEMP. Generated bindings не изменились в Git.
- Реальный desktop: запуск именно собранного EXE с `--smoke-port 9223`, затем `node scripts/gui-modern-smoke.mjs` — FAIL, exit 1 `Desktop WebView2 unavailable`. CDP/DOM/native comparison не достигнуты; процесс, запущенный проверкой, остановлен. Это не доказательство ошибки прикладного кода и не PASS UI. Исторический 0x8000FFFF этой попыткой не подтверждён.
- `git diff --check` проверяется перед финальным коммитом. Runtime логи/coverage/build outputs в ignored local/cache/bin, не добавлены в Git.

## Непроверенное и ограничения

Изменяющие установки ПО/драйверов/принтеров, SMB server credentials, реальные политики/настройки, reboot и Windows 10/LTSC VM не выполнялись: изолированной VM/тестового сервера нет. Legacy/Win7 не проверялась намеренно. Race detector, dependency CVE/freshness scan и GitHub CI в этой итерации не запускались. Remote отсутствует, release/tag не публиковались. Старые ZIP не перепакованы и не содержат новый fix — не использовать их как текущий release.

## Неисправленные проблемы

- P0-02: deploy не подключает GuardAdapter; требования профиля могут игнорироваться при выполнении.
- P0-03: reboot Verify пропускается, Success разрешает зависимости и сохраняется Resume без перепроверки.
- P0-04: обязательные эффекты/предупреждения теряются в deploy; browser/settings не имеют полноценного read-back.
- P0-05: package/deploy операции не сериализованы общим gate, ResumeDeployment имеет окно конкурентного запуска.
- P0-06: запуск под другим администратором меняет user context HKCU/Desktop/CredMan; worker с исходным SID отсутствует.
- Другие gaps: post-update target version, semantic profile parameters/check_ref, TCP/IP printer port, process-tree cancellation, ACL/bounded session parsing, durable logs. Подробности и степень подтверждения — AUDIT.

## Изменения архитектуры

Перепроектирования нет. Единственное runtime изменение — fail-closed post-install detection. Стратегия и документы приведены к существующим трём модулям и реальным bridge methods. Root Go version, Legacy code/tests и workflows не изменялись. Требования к отдельному Modern release gate и user-context isolation пока план, не реализованная архитектура.

## Git и артефакты

Ветка `master`, remote отсутствует. Перед итоговым commit изменены только перечисленные документационные файлы; fix уже отдельным коммитом. После финального commit обязательно выполнить `git status --short` и сообщить фактический результат. Не коммитить local/cache/bin/dist, логи, inventory и персональные данные.

## Следующие 2–4 задачи

Следующая конкретная задача — P0-02 runtime applicability на start/resume (единая граница deploy, fresh system snapshot, несовместимый/unknown target никогда не вызывает mutation).

Затем P0-05 общий mutation gate с конкурентными service tests и P0-03 честная reboot boundary с persistence/dependency/report tests. Точные критерии в таблице TASKS.md. Если reboot задача не помещается, завершить первые две и передать третью открытой. Новых функций/Legacy work не начинать.

## Точные инструкции следующему агенту

1. `git status`, `git branch --show-current`, `git log -20 --oneline`; AGENTS/HANDOFF/TASKS/README, docs/architecture.md и разделы P0-02/03/05 AUDIT. Не повторять полный аудит.
2. Изучить core/deploy/runner.go, adapter.go, core/inventory/applicability.go, CLI/service wiring и имеющиеся tests. Сначала написать regression для несовместимого start/resume; обеспечить отсутствие mutation calls.
3. Для доступных caches и временных файлов (из repo root):

```powershell
New-Item -ItemType Directory -Force local/audit-temp | Out-Null
$env:GOCACHE = Join-Path $PWD 'cache/go-build'
$env:GOMODCACHE = Join-Path $PWD 'cache/_go-mod'
$env:TEMP = Join-Path $PWD 'local/audit-temp'
$env:TMP = $env:TEMP
$env:GOTMPDIR = $env:TEMP
go test ./core/deploy ./core/inventory ./cmd/windeploy-run -count=1
go vet ./core/deploy ./core/inventory ./cmd/windeploy-run
# При изменении service/bridge: scripts/build-modern.ps1
# Полная безопасная root проверка, только при необходимости:
go test ./... -count=1 -skip 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip'
```

4. Для native DLL/COM и Wails может понадобиться разрешённый запуск вне sandbox. Не обходить сбой фиктивным результатом; сначала local TEMP. Desktop smoke требует действительного интерактивного WebView2, а не только собранного EXE.
5. По каждой законченной задаче — test/vet/build по объёму, diff review, статус и commit. В конце HANDOFF и чистый Git. Полная Win7 матрица/установка старого toolchain не требуются.

## Опасные действия

Не запускать реальные установщики/драйверы/принтеры/SMB/политики/изменения Windows на хосте. Только изолированная VM с согласованными правами/ресурсами. Не отключать trust/UAC/Defender/Firewall. Не считать 0600 достаточной ACL-защитой и не исполнять изменённую недоверенную session под admin. Не публиковать текущий preview как production-ready.
