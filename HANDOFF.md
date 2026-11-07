# WinDeploy — HANDOFF

## Текущий этап и факты

2026-11-08: первый публичный **Pre-release v0.1.0-beta.1** опубликован:
https://github.com/ashtray01/WinDeploy/releases/tag/v0.1.0-beta.1
Это beta, не Stable. Windows 11 x64 — проверенная платформа; Win10/LTSC не
проверены. Windows 7 Legacy FROZEN и в Modern-артефакты не входит.
Владелец явно разрешил публикацию в этой задаче. B1/B2 остаются открытыми
непроверенными сценариями, но по решению владельца не являются общим запретом
выпуска. Security/runtime gates не ослаблялись; полного аудита не было.

## Завершённые задачи

1. Проверены HEAD/origin/VERSION, отсутствие прежних тегов/Releases до выпуска,
   MIT, существующий CI и build/package scripts. В release workflow добавлен
   отсутствовавший secret preflight с Git history; остальная pipeline сохранена.
2. CI PASS → annotated tag → release workflow → проверка draft/downloads →
   публичный Pre-release. Подпись Git-тегов не настроена, тег annotated unsigned.
3. Документация обновлена отдельным коммитом после выпуска; release tag не менялся.

## Коммиты

- Исходный HEAD: `71c56bdc0d7c495c7368ba6b7519d2da58c3c2f7`.
- `fb2920c` — fix(release): gate beta artifacts with history secret preflight.
- `56fe16351720b4a74a5e773a7a26e270fe4bbf84` — docs(release): align packaged
  limitations with owner approval; **release commit**.
- Финальный документационный коммит этой итерации: `git log -1 --oneline`.
  Он находится после тега и не входит в выпущенные EXE/ZIP.

## Фактические проверки

- Modern CI: https://github.com/ashtray01/WinDeploy/actions/runs/42421280314
  — PASS на release commit, все 3 jobs: Core Go 1.20.14 amd64+386 test/vet/build
  и CLI builds; Modern Go 1.26.2 tests/vet + frontend tests/strict typecheck/build;
  production Wails EXE, packaging/checksums/upload.
- Release: https://github.com/ashtray01/WinDeploy/actions/runs/42421767518
  — PASS на том же commit: history secret preflight, tag/VERSION, Core checks,
  Modern tests/typecheck/build/vet, EXE, ZIP/SHA256, upload и draft creation.
- Локально: verify-version PASS; history secret preflight — 0 blocking findings.
  Manual items проверены: test fixtures/private IP/fake user path и термин
  OneDrive/antivirus в документации; значения не являются credentials.
- Скачаны все 4 draft assets, сверены SHA-256. EXE ProductVersion/FileVersion
  0.1.0-beta.1, embedded commit соответствует тегу, Authenticode NotSigned;
  production EXE не содержит smoke-port/remote-debugging-port.
- ZIP: EXE совпадает с отдельным asset; BUILD-INFO содержит правильные commit,
  version, windows/amd64; MIT и release notes на месте; diagnostic/Legacy EXE
  и runtime logs не включены. Release body совпадает с RELEASE-NOTES.md.
- После публикации: публичный API подтверждает draft=false, prerelease=true;
  HTML release page доступна, все 4 assets скачаны без авторизации и повторно
  сверены. Хеши — в docs/RELEASE_VERIFICATION_v0.1.0-beta.1.md.
- Исторический Windows 11 GUI smoke (до этой итерации): actual diagnostic
  DOM/bridge/native Inventory PASS, pageErrors=0, два viewport; production
  window/startup log и безопасный WebView2 fallback проверены. В этой итерации
  runtime/UI не менялись; скачанный release EXE не запускался. CI build не
  выдаётся за новый actual desktop/bridge smoke.
- Не выполнялись: package install/update/cancel acceptance, SMB server/1219,
  реальный reboot/fast startup, drivers/printers/policies/settings mutation,
  Windows 10/LTSC, Windows 7. Никакие установки/системные изменения на хосте
  ради выпуска не выполнялись.

## Открытые и закрытые blockers

- **B1 открыт:** real package-install/update/cancel acceptance отсутствует.
- **B2 открыт:** SMB server/credentials/1219 acceptance отсутствует.
- B1/B2 — явные ограничения опубликованной beta по решению владельца, не PASS.
- **B3 закрыт:** MIT и ashtray01/WinDeploy.
- **B4 закрыт для среза:** local + release CI history preflight, 0 blocking.
- **B5:** предыдущий actual Windows 11 GUI smoke сохранён; нового smoke нет.
- **B6 закрыт:** CI/release PASS, draft и публичные downloads проверены.
- **B7:** scope только Windows 11 x64, не подтверждение Win10/LTSC.
- **B8 закрыт:** явное разрешение владельца на этот публичный Pre-release.
- Критических сбоев безопасности/сборки/артефактов/лицензирования в рамках
  проверок выпуска не обнаружено; это не полный security audit.

## Что включено в beta / экспериментально / отключено

Inventory, профили/DAG/preview, каталог, выполнение и отчёты, read-only network
diagnostics включены. Package-install и SMB доступны, но реальная приёмка
не подтверждена. Drivers/printers/browser policies/Windows settings —
экспериментальные. Reboot continuation не VM-accepted. Нет auto-update,
code signing, elevated worker. Legacy исключён из Modern release.

## Архитектура и изменения

Runtime/UI не менялись. Root остаётся Go 1.20, Modern изолирован в
desktop/modern (Go 1.26.2, Wails v3.1.0). VERSION — единый источник версии;
internal/buildinfo и Windows resources получают version/commit при сборке.
Preview не заменяет runtime SID/session/ACL/installer trust gates.
Session storage bounded/strict/ID-bound, required effects fail closed;
owner/ACL не аутентифицирует автора и не защищает от same-user/admin tampering.
WebView2 fallback сохраняет owner/write-ACE/reparse/write-probe gates;
read-only ACE допустим, unknown/mismatch SID отклоняется. Release workflow
по-прежнему создаёт только draft; публикация выполнена отдельным API действием
после проверки и в пределах разрешения владельца. Legacy не изменялся.

## Git и артефакты

Локальная ветка master, upstream origin/main. origin:
https://github.com/ashtray01/WinDeploy.git. Annotated tag v0.1.0-beta.1 указывает
на 56fe16351720b4a74a5e773a7a26e270fe4bbf84; не перемещать/не переписывать.
EXE/ZIP/logs/скачанные assets и вспомогательные release scripts хранятся только
в ignored local/release-verification; в Git не добавлялись. Пользовательский
untracked temp/windeploy-icon.png не изменялся. После docs commit допустим
только `?? temp/`. Portable docs — срез release commit, последующие уточнения
находятся в main; это не повод перезаливать assets или менять тег.

## Следующие 2–4 задачи для v0.1.0-beta.2

1. B1: реальный package-install acceptance на одноразовой Win11 VM либо с
   отдельным разрешением владельца — detection/repeat/update/failure/cancel и
   отчёт, без удаления чужих программ.
2. B2: реальный SMB acceptance — тестовый сервер/учётная запись, required
   shortcut, повтор/1219, отсутствие credentials в logs/report.
3. Исправления по воспроизводимым GitHub Issues; следующий релиз — отдельной
   задачей, без новых крупных модулей и без Legacy roadmap.

## Точные команды

```powershell
git status --short
git log -1 --oneline
git rev-parse 'v0.1.0-beta.1^{commit}'
./scripts/verify-version.ps1
./scripts/check-release-secrets.ps1 -IncludeHistory
./scripts/test-core.ps1 -Go <path-to-go1.20.14> -BuildCache ci-go120
./scripts/build-modern.ps1 -Diagnostic
# Скачивание опубликованных checksums без авторизации:
Invoke-WebRequest https://github.com/ashtray01/WinDeploy/releases/download/v0.1.0-beta.1/SHA256SUMS.txt -OutFile local/SHA256SUMS.txt
# SHA-256 скачанного EXE:
Get-FileHash local/release-verification/public/WinDeploy-Windows-x64.exe -Algorithm SHA256
```

Тесты с native writes — только disposable runner/VM либо с разрешением.
GUI smoke использует diagnostic build и node scripts/gui-modern-smoke.mjs;
production EXE не включает CDP flags. При ACL/temp/OneDrive ограничениях тест
не считать пройденным и глобальные настройки Windows не менять.

## Опасные действия и ограничения

Read-only Inventory/diagnostics допустимы. Real install/SMB/drivers/printers/
policies/settings/reboot — только VM либо отдельное разрешение. Не отключать
Defender/UAC/Firewall и не менять чужие ACL. EXE не подписан; SmartScreen может
предупреждать. Не обещать rollback или остановку дочерних installer processes.
Session storage требует trusted local storage и Windows ACL; 0600 недостаточно.

## Условия разрешения публикации

Для v0.1.0-beta.1 выполнены: MIT/URL, история secret preflight, CI PASS,
release workflow PASS, draft и скачанные checksums, явное разрешение владельца,
публичный prerelease и downloads. B1/B2 честно раскрыты, не объявлены закрытыми.
Разрешение относилось к этому релизу; второй релиз не создан. Исходный тег и
опубликованные assets не переписывать. Для последующих релизов — отдельная
задача и актуальная проверка gates.
