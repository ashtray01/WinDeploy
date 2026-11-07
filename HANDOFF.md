# WinDeploy — HANDOFF

## Решение владельца о выпуске — 2026-11-08

Текущая задача явно разрешает публичный Pre-release v0.1.0-beta.1 после
успешного CI, проверки draft и SHA-256 скачанных assets. Это решение заменяет
прежний запрет binary Release и требование B1/B2 как общего release gate.
B1/B2 не выполнены: package-install/SMB acceptance остаются открытыми
ограничениями beta, не подтверждением безопасности или E2E. Runtime gates
не изменяются. B8 закрыт разрешением; B6 ожидает release workflow и assets.
В release workflow добавлен существующий secret preflight с Git history;
сборка/typecheck/tests/vet/packaging сохранены. Локально version gate PASS,
secret preflight — 0 blocking; manual items остаются test/doc fixtures.


## Текущий этап и факты

2026-11-08: исправлен фактический startup blocker **v0.1.0-beta.1**. Владелец
выбрал MIT и явно разрешил первый source push в публичный
`https://github.com/ashtray01/WinDeploy`; `main` опубликован. GitHub Modern CI
run `42419867600` для `619ca04` — PASS. Binary Release не разрешён и не готов:
B1 (real install), B2 (SMB server) и проверка draft открыты. Windows 11 x64 — единственная заявленная
платформа; Win10/LTSC не проверялись. Windows 7 Legacy FROZEN. Root Go 1.20
сохранён, Modern Go 1.26/Wails v3.1.0. Полный аудит не повторялся.

## Завершённые задачи

1. **Beta Readiness / перепланирование.** Создан `docs/BETA_READINESS.md`:
   матрица функций, scope только Win11 x64, B1–B8, что включено/экспериментально/
   отключено. `TASKS.md` переклассифицирован в BETA BLOCKER / BETA REQUIRED /
   POST-BETA / FROZEN. `AGENTS.md` закрепляет Public Beta, отказ от обязательного
   общего VM acceptance и запрет публикации без владельца.
2. **Security release gates + stability.** Подтверждены существующие session
   trust/bounded JSON/ID binding/owner/write-ACE (`core/trust`), SID gate,
   installer SHA-256/Authenticode и required-effect read-back. Добавлены:
   `internal/atomicfile` с ограниченным retry transient Windows sharing/AV
   конфликтов, интеграция в state/session/download/portable, negative test
   «report не содержит task parameters/password», `scripts/test-core.ps1` с
   контролируемым temp и allowed native-write switch. Elevated worker и
   impersonation отсутствуют; unknown/mismatch fail closed.
3. **Core journey.** Execution требует preview плана и подтверждение; Legacy не
   выбирается по умолчанию; Packages не выбирает пакеты автоматически, требует
   confirm, не показывает зелёный `Завершено` при ошибке/пустом запуске; polling
   потеря связи показывается как stale. Версия в UI/EXE — из одного `VERSION`.
   Actual GUI EXE smoke PASS.
4. **Release infrastructure.** Modern-only `ci.yml`; `release.yml` создаёт только
   **draft** pre-release по тегу/`workflow_dispatch`; Legacy CI manual/frozen.
   `verify-version.ps1`, `package-modern-release.ps1` (EXE/ZIP/SHA256SUMS),
   `check-release-secrets.ps1`, `VERSION`, EXE version resource, docs/issue
   templates/release notes/publication checklist.
5. **Secret preflight.** `check-release-secrets.ps1 -IncludeHistory` на текущем
   срезе: 0 blocking findings; manual-review items — тестовые/документационные
   примеры private IP и fake `C:\Users\Test`. Значения не печатались. Повторить
   после финального docs commit и перед открытием репозитория.
6. **Фактический startup fix.** Ошибка из `%LOCALAPPDATA%\WinDeploy\logs\startup.log`
   воспроизведена: read-only `(RX)` ACE ошибочно попадал под составную
   `FILE_GENERIC_WRITE`. ACL gate теперь проверяет только реальные write/append/
   delete/change-ACL права; read-only ACE разрешён. Write probe и reparse/owner
   gates сохранены. Каждый отклонённый WebView2 candidate пишет причину в startup
   log. Default-path diagnostic startup создал WebView2 Environment через
   доверенный portable fallback, actual DOM/bridge smoke PASS (`740a3e4`).
7. **Source publication preparation.** MIT выбран владельцем; README links
   заменены на `ashtray01/WinDeploy`; source опубликован в `main`. Первый CI
   выявил Windows runner owner/short-path и relative Wails path; исправлены без
   ослабления ordinary-user write gates. Финальный CI полностью PASS. Release
   не создавался и не публиковался.

## Коммиты

Ветка `master`; `origin` настроен на `ashtray01/WinDeploy`; тегов нет. Runtime/code коммиты этой
итерации:

- `ba09bc6` — fix(stability): retry transient state replacement and isolate test temp.
- `96e9fb1` — fix(modern): require plan preview and explicit package confirmation.
- `2e7dbc6` — feat(release): unify beta identity and add Modern-only pipeline.
- `b032eb8` — docs: prepare public beta materials and anonymized screenshots.
- `a307147` — docs: record beta readiness handoff and open blockers.
- `9e36f03` — fix(release): avoid false positive in secret preflight.
- `662acb9` — fix(modern): label experimental beta surfaces explicitly.
- `5ca5dbf` — fix(release): compile remote debugging out of production EXE.
- `f90b7bf` — fix(modern): keep startup log when stderr is unavailable.
- `603919c` — fix(modern): offer admin relaunch when WebView2 cache is limited.
- `95b1b77` — fix(modern): harden WebView fallback and remove startup UAC.
- `ea7caef` — docs: secure WebView fallback/без startup UAC.
- `2f63524` — test(modern): pin TaskDialog packed layout.
- `740a3e4` — fix(modern): accept read-only ACLs for WebView cache.
- `8b631b7` — test(trust): own private ACL fixtures on CI.
- `6448e11` — fix(trust): handle elevated owners and short Windows paths.
- `619ca04` — fix(ci): resolve the pinned Wails path before building.
- Текущий HEAD после docs/test-коммитов — `git log -1 --oneline`; runtime EXE
  локально пересобран из `6448e11`; CI собрал `619ca04`. Release workflow
  пересоберёт EXE из тега.

## Фактические проверки

Среда: текущая Windows 11 x64, Go 1.26.2, локальный Go 1.20.14, Node 24.13.0,
Wails v3.1.0, WebView2 157.0.4341.42. Зависимости/toolchains не устанавливались.

- **Root Go 1.20.14:** `scripts/test-core.ps1 -Go <go1.20.14>` — PASS для amd64 и
  386: full `go test ./... -count=1` (кроме двух native write tests по умолчанию),
  `go vet ./...`, `go build ./...`. В финальном прогоне использовался workspace
  `cache/tmp`; ACL trust tests PASS. `-AllowNativeWrites` не применялся на хосте.
- **Modern Go 1.26.2:** `go test ./... -count=1` и `go vet ./...` — PASS.
- **Frontend:** 46/46 tests в 16 файлах, strict `tsc --noEmit`, production Vite
  build — PASS.
- **EXE:** `scripts/build-modern.ps1 -Diagnostic` — PASS. `bin/WinDeploy-Windows-x64.exe`,
  ProductVersion/FileVersion `0.1.0-beta.1`, company/product `WinDeploy`,
  commit `95b1b77b6f8c56491c7d4e0a4dcd7e57136009b8`, icon/manifest/production
  assets. Production EXE не содержит строк/флага `smoke-port` и
  `remote-debugging-port` (проверено поиском по PE); DevTools выключен. Remote
  debugging есть только в непакетируемой диагностической сборке с тегом `smoke`.
  `Get-AuthenticodeSignature` — **NotSigned** (сертификата нет; не заявлять
  подпись). Startup log production EXE проверен: файл непустой, startup/Wails
  evidence записаны.
- **WebView2 recovery:** при недоступном `%LOCALAPPDATA%\WinDeploy\WebView2`
  production EXE показал TaskDialog «WinDeploy — WebView2» с кнопками
  «Продолжить с резервным каталогом» и «Выход» (без UAC). «Продолжить» запустило
  приложение с `bin\WinDeploy-WebView2Data`, создало окно и startup log, записало
  marker; повторный запуск использовал тот же каталог без диалога. Автоматический
  fallback проверяет owner/ACL (без записи для посторонних), отсутствие reparse
  точки и write probe. Unit-тесты отклоняют каталог с записью для Users,
  пропускают Program Files и проверяют повторное использование/сохранение файла.
  Реальный запуск из Program Files и с USB/FAT не выполнялся: при недоступном
  ACL такой fallback должен быть отклонён, а LocalAppData/Temp использованы.
  Fallback-порядок покрыт Go-тестами; существующий каталог не удаляется.
- **Actual GUI smoke:** запуск `bin/WinDeploy-Diagnostic-x64.exe` (console + `smoke`
  tag) с отдельными `--webview-data-dir`, `--log-dir`, `WINDEPLOY_STATE_DIR`,
  `--smoke-port 9223`; `node scripts/gui-modern-smoke.mjs` — PASS: `actualDesktop=true`,
  native hostname/app count matched, rescan, inventory tables, DAG, catalog,
  diagnostics, shares/drivers/printers/browsers/windows pages, execution page,
  journal, viewports 1366×768/1024×640, `pageErrors=0`. UI/service code тот же;
  production EXE дополнительно проверен запуском/окном, но не CDP. Это не ручное
  подтверждение окна владельцем.
- **Startup ACL regression:** реальные Windows ACL tests PASS: чужой `(RX)`
  принимается, `(W)/(M)` отклоняется. Ограниченный агентный token не может писать
  в `%LOCALAPPDATA%`, поэтому default-path smoke ожидаемо выбрал доверенный
  `bin\WinDeploy-WebView2Data`; WebView2 Environment создан, GUI smoke повторно
  PASS, `pageErrors=0`. Это доказывает recovery/ACL fix, но не ручной видимый
  запуск владельцем нового EXE. Полный root test/vet/build amd64+386 на
  Go 1.20.14 подтверждён финальным GitHub CI.
- **GitHub CI:** [run 42419867600](https://github.com/ashtray01/WinDeploy/actions/runs/42419867600)
  — PASS. Core Go 1.20.14 test/vet/build amd64+386, Modern Go 1.26 + React,
  Wails EXE, package/SHA-256 dry-run и artifact upload прошли на disposable
  Windows runner. Draft Release не создавался.
- **Packaging:** `scripts/package-modern-release.ps1 -SkipBuild` — PASS. Артефакты
  (candidate, не публиковать):
  - `dist/WinDeploy-Windows-x64.exe` SHA-256 `f9021f9bdb52b50e7141cebd75a3e39f5ad6c3d074f9c7cf360e168894e38ba2`
  - `dist/WinDeploy-0.1.0-beta.1-portable-x64.zip` SHA-256 `c4dca86220d2a871121e16d3c252573d946396cbda034ec5e26f05339af5efdf`
  - `dist/RELEASE-NOTES.md` SHA-256 `33a14171b6e848f9db6392664a7b03b602aa971a5f9f451dc90b646c3f567f00`
  - `dist/SHA256SUMS.txt` с этими хешами; повторная проверка скриптом PASS.
- **Version gate:** `scripts/verify-version.ps1` — PASS.
- **Secret scan:** 0 blocking; manual items review. См. выше.
- **Не выполнялось:** реальная установка/обновление/отмена пакета, SMB-сервер с
  credentials, реальный reboot/fast startup, драйверы/принтеры, browser/Windows
  policies, Windows 10/LTSC, GitHub-hosted Actions, публичный Release. Не
  выдавать это за проверенное.

## Открытые blockers

- **B1** — нет реальной установки одного безопасного пакета и negative install
  acceptance. Нужно разрешение владельца, согласованный пакет (например 7-Zip),
  detection/repeat/update/failure/cancel и отчёт.
- **B2** — нет SMB-сервера/учётной записи для проверки connect/credentials/
  required shortcut/1219.
- **B3 закрыт** — владелец выбрал MIT, URL/README/origin зафиксированы.
- **B6 частично закрыт** — source и GitHub CI PASS; draft Release не запускался.
- **B8 частично закрыт** — source push явно разрешён; binary Release требует
  отдельного разрешения после B1/B2/B6.
- B5 закрыт для этого EXE; UI-изменения требуют нового smoke. B7 закрыт scope-
  решением (только Win11 x64), а не проверкой Win10.

## Что включено в beta / экспериментально / отключено

- Включено: Inventory, профили/DAG/preview, каталог/установка выбранного ПО,
  прогресс/отмена/результат, network diagnostics, SMB (после B2), session/resume
  без false success, HTML/JSON report, Modern GUI.
- Экспериментально (с предупреждением в UI/документах): драйверы, принтеры,
  browser policies, Windows rename/timezone/Explorer/power. Без VM-приёмки; не
  заявлять production-ready.
- Не включено/FROZEN: Legacy Windows 7; Win10/LTSC не заявляются; нет auto-update,
  code signing, elevated worker, полного reboot continuation acceptance.
- Опасные сценарии, которые нельзя безопасно завершить, не маскируются
  успешным статусом: SID/trust/installer gates fail closed.

## Архитектура и изменения

- `internal/buildinfo` — единый build identity; версия внедряется `-X`; UI
  получает `GetAppInfo`, EXE — Windows version resource. `VERSION` — canonical.
- `internal/atomicfile` — bounded retry только transient Windows ошибок; не
  заменяет ACL и не гарантирует power-loss durability.
- `core/deploy.Store` — bounded strict JSON, ID binding, owner/ACL trust перед
  mutation; session resume не получает elevated worker/impersonation.
- `core/trust` отличает реальные write/delete/change-ACL права от общих
  `READ_CONTROL`/`SYNCHRONIZE`; read-only ACE не снижает доверие, write ACE
  постороннего по-прежнему fail closed. Owner — current SID или встроенная
  Administrators для elevated default-owner; это не расширяет уже доверенную
  admin write-модель.
- WebView2 reparse gate проверяет атрибут каждого ancestor и не принимает
  нормальное 8.3→long-name разрешение за junction.
- `deploy.Adapter`/engine — preview не заменяет runtime gate; warnings
  сохраняются; required effect fail closed.
- Modern release workflows separated from Legacy; release creates draft only.
- Legacy sources/tests/docs не менялись.

## Git и артефакты

`master` tracking `origin/main`, публичный source опубликован; тегов/Release нет.
Runtime artifacts/logs/EXE/ZIP/caches не
коммитились. `dist/` и `bin/` ignored. Пользовательский untracked
`temp/windeploy-icon.png` не изменялся/не добавлялся/не удалялся. Скриншоты
`docs/images/` — из настоящего GUI, hostname заменён на `DEMO-WORKSTATION`.
После финального docs commit проверить `git status --short`; допустим только
`?? temp/`.

## Следующие 2–4 задачи

1. **B1**: с разрешения владельца установить один безопасный пакет на Win11 x64;
   проверить detection/repeat/update/failure/cancel и отчёт. Не удалять
   существующие программы.
2. **B2**: тестовый SMB-сервер/учётная запись; connect, required shortcut,
   повтор, 1219, отсутствие пароля в report/log.
3. **B6/Release**: после B1/B2 и отдельного разрешения создать только draft,
   проверить assets/SHA-256; Publish не нажимать автоматически.
4. После следующего runtime-коммита снова собрать candidate, выполнить actual
   GUI smoke и обновить SHA-256.

## Точные команды

```powershell
# Root Go 1.20
./scripts/test-core.ps1 -Go <path>\go1.20.14.exe -BuildCache ci-go120

# Modern build + tests + EXE
./scripts/build-modern.ps1 -Diagnostic

# Release identity/package
./scripts/verify-version.ps1
./scripts/package-modern-release.ps1 -SkipBuild
Get-Content dist/SHA256SUMS.txt
Get-AuthenticodeSignature dist/WinDeploy-Windows-x64.exe

# Actual GUI smoke: сначала build-modern -Diagnostic, затем diagnostic EXE
# с --smoke-port 9223; production EXE этот флаг не принимает.
# node scripts/gui-modern-smoke.mjs

# Secret preflight
./scripts/check-release-secrets.ps1 -IncludeHistory
```

`Wails` pinned `cache/tools/wails-3.1/wails3.exe`. Не устанавливать новый
toolchain без необходимости. Если файловая песочница запрещает DACL, ACL-тесты
не считать пройденными; запускать в обычной оболочке/CI.

## Опасные действия и ограничения

Read-only Inventory/диагностика допустимы. Реальная установка, SMB credentials,
драйверы, принтеры, policies/settings/reboot — только VM или отдельное разрешение
владельца. Не отключать Defender/UAC/Firewall, не менять чужие ACL/профили, не
убивать пользовательские процессы. EXE не подписан; SmartScreen предупреждение
ожидаемо. Source push разрешён после MIT/preflight; binary Release не публиковать:
CI, B1/B2 и отдельное разрешение владельца не закрыты.

## Условия разрешения публикации

Публичный Release/repository разрешается только когда одновременно выполнено:

1. **Выполнено:** владелец выбрал MIT, заменил `LICENSE`, зафиксирован URL,
   README больше не содержит `OWNER`.
2. B1 и B2 закрыты воспроизводимыми логами/тестами; все открытые blockers
   перечислены в release notes.
3. `check-release-secrets.ps1 -IncludeHistory` на финальном дереве не имеет
   blocking findings; manual items проверены; при секрете ключ ротирован.
4. Modern CI PASS на GitHub disposable runner; release workflow создал draft,
   SHA-256 и assets проверены.
5. Владелец отдельно подтвердил публикацию (B8); только после этого нажать
   Publish и объявить релиз.
