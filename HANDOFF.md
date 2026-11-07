# WinDeploy — HANDOFF

## Текущий этап и факты

2026-11-08: первая итерация подготовки **v0.1.0-beta.1** завершена. Это public
beta candidate, **не опубликован и не готов к публикации**: B1 (real install),
B2 (SMB server), B3 (license/public URL) и B6 (GitHub CI/draft) открыты, B8
(явное разрешение владельца) не выдано. Windows 11 x64 — единственная заявленная
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

## Коммиты

Ветка `master`; remote отсутствует; тегов нет. Runtime/code коммиты этой
итерации:

- `ba09bc6` — fix(stability): retry transient state replacement and isolate test temp.
- `96e9fb1` — fix(modern): require plan preview and explicit package confirmation.
- `2e7dbc6` — feat(release): unify beta identity and add Modern-only pipeline.
- `b032eb8` — docs: prepare public beta materials and anonymized screenshots.
- `a307147` — docs: record beta readiness handoff and open blockers.
- `9e36f03` — fix(release): avoid false positive in secret preflight.
- `662acb9` — fix(modern): label experimental beta surfaces explicitly.
- Финальный docs commit с обновлёнными хешами — `git log -1 --oneline`; runtime
  не меняет. Release workflow пересоберёт EXE из тега.

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
  commit `662acb98f533077005d0965ddc543a0d1455c666`, icon/manifest/production
  assets. `Get-AuthenticodeSignature` — **NotSigned** (сертификата нет; не
  заявлять подпись).
- **Actual GUI EXE smoke:** запуск с отдельными `--webview-data-dir`, `--log-dir`,
  `WINDEPLOY_STATE_DIR`, `--smoke-port 9223`; `node scripts/gui-modern-smoke.mjs`
  — PASS: `actualDesktop=true`, native hostname/app count matched, rescan,
  inventory tables, DAG, catalog, diagnostics, shares/drivers/printers/browsers/
  windows pages, execution page, journal, viewports 1366×768/1024×640,
  `pageErrors=0`. Это automated DOM/bridge smoke, не ручное подтверждение окна.
- **Packaging:** `scripts/package-modern-release.ps1 -SkipBuild` — PASS. Артефакты
  (candidate, не публиковать):
  - `dist/WinDeploy-Windows-x64.exe` SHA-256 `938eba700d75adddb90c392dd36a007c8e569520bba7e1dccde2d8b3d0322ec2`
  - `dist/WinDeploy-0.1.0-beta.1-portable-x64.zip` SHA-256 `1853bbbe62a08b8d5a6dccf02f85b0c28520142736318aae3a9ca2150ffea3e8`
  - `dist/RELEASE-NOTES.md` SHA-256 `debe5ad112b95b42b9cb2c4e88c6360e962163537097602522c956777528346e`
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
- **B3** — лицензия не выбрана; `LICENSE` — заглушка. Нет публичного owner/repo
  URL; README использует `OWNER` placeholder.
- **B6** — нет remote и GitHub CI/draft Release. Workflows не запускались на
  GitHub.
- **B8** — нет явного разрешения владельца на публичный repo/Release.
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
- `deploy.Adapter`/engine — preview не заменяет runtime gate; warnings
  сохраняются; required effect fail closed.
- Modern release workflows separated from Legacy; release creates draft only.
- Legacy sources/tests/docs не менялись.

## Git и артефакты

`master`, remote отсутствует, тегов нет. Runtime artifacts/logs/EXE/ZIP/caches не
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
3. **B3/B6/B8**: license + owner/repo URL, remote, GitHub CI, draft Release,
   затем явное решение владельца о публикации.
4. Повторить `check-release-secrets.ps1 -IncludeHistory` на финальном дереве;
   пересобрать candidate после каждого runtime-коммита и обновить хеши.

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

# Actual GUI smoke (запускать GUI отдельно с --smoke-port 9223)
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
ожидаемо. Не публиковать repo/Release: лицензия, secret scan, CI и B1/B2/B8 не
закрыты.

## Условия разрешения публикации

Публичный Release/repository разрешается только когда одновременно выполнено:

1. Владелец выбрал лицензию и заменил `LICENSE`; зафиксирован owner/repo URL,
   README больше не содержит `OWNER`.
2. B1 и B2 закрыты воспроизводимыми логами/тестами; все открытые blockers
   перечислены в release notes.
3. `check-release-secrets.ps1 -IncludeHistory` на финальном дереве не имеет
   blocking findings; manual items проверены; при секрете ключ ротирован.
4. Modern CI PASS на GitHub disposable runner; release workflow создал draft,
   SHA-256 и assets проверены.
5. Владелец отдельно подтвердил публикацию (B8); только после этого нажать
   Publish и объявить релиз.
