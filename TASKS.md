# WinDeploy — controlled development (Public Beta)

## Текущий этап

Цель — **v0.1.0-beta.1**, Modern Windows 11 x64. Public Beta, не stable. Windows 10/
LTSC не заявляются без проверки. Windows 7 Legacy FROZEN. Одна итерация — 2–4
задачи с тестами и коммитами.

Текущий срез — beta candidate. Владелец выбрал MIT и разрешил отправить исходный
код в `ashtray01/WinDeploy`. Публикация binary Release не разрешена: остаются
B1/B2/B6 из [BETA_READINESS](docs/BETA_READINESS.md).
Release workflow создаёт только draft. VM acceptance больше не является
обязательным общим критерием, но конкретные изменяющие сценарии без проверки не
заявляются.

## Сделано в этой итерации

- [x] **Beta Readiness**: матрица, scope, blockers, отказ от полного аудита;
  TASKS переклассифицирован; AGENTS закрепляет Public Beta.
- [x] **Security release gates**: session trust/bounded JSON/owner ACL и SID gate
  подтверждены тестами; добавлены bounded atomic rename, безопасная работа с
  temp и negative tests. Elevated worker/impersonation нет; unknown/mismatch
  fail closed (`ba09bc6`).
- [x] **Core journey**: preview-before-execute, explicit package selection and
  confirmation, честные failed/empty статусы, потеря polling видна, Legacy не
  выбирается по умолчанию, version в UI/EXE, startup log не теряет запись при
  недоступном stderr, WebView2 fallback с безопасным ACL/reparse gate и выбором
  «Продолжить»/«Выход» без UAC. Actual diagnostic/smoke GUI smoke PASS (native
  hostname/app count, два viewport, pageErrors=0) (`96e9fb1`, `f90b7bf`,
  `95b1b77`).
- [x] **Release infrastructure**: `VERSION`, version gate, Modern-only CI,
  Modern release workflow (draft pre-release), SHA256SUMS/portable ZIP,
  `verify-version.ps1`, `package-modern-release.ps1`, production EXE без
  remote-debug flag (`2e7dbc6`, `5ca5dbf`).
- [x] **Public docs**: README со screenshots, MIT LICENSE, CHANGELOG,
  CONTRIBUTING, SECURITY, KNOWN_ISSUES, release notes, issue templates,
  publication checklist (`b032eb8`).
- [x] **Secret preflight**: tracked files + Git history проверены; blocking
  findings 0, manual items — тестовые/документационные примеры. Повторить после
  финального docs commit.
- [x] **Startup ACL regression**: составная `FILE_GENERIC_WRITE` больше не
  принимает общие read/synchronize-биты за право записи; `(RX)` разрешён,
  реальные write/delete/ACL-права посторонних по-прежнему fail closed. Свежий
  default-path diagnostic startup и GUI bridge smoke PASS (`740a3e4`).
- [ ] B1/B2/B6 ещё не закрыты. Не считать документацию их заменой.

## BETA BLOCKER

| ID | Блокер | Доказательство | Критерий закрытия |
|---|---|---|---|
| B1 | Реальная установка одного безопасного пакета не выполнена | Есть unit/service/UI, EXE собран; install не запускался | С разрешения владельца: 7-Zip или другой согласованный пакет; detection до/после, повтор, update, failure/cancel, отчёт |
| B2 | SMB-сервер и credentials context не проверены | Native list/.lnk read-back PASS; сервера нет | Тестовый SMB: connect, shortcut, repeat, 1219, no secret in report |
| B3 | ~~Лицензия/URL не выбраны~~ | **Закрыт:** владелец выбрал MIT; URL `https://github.com/ashtray01/WinDeploy`; `origin` настроен, README обновлён | Первый public source push после финального preflight |
| B4 | ~~Git-история не проверена на секреты/данные~~ | **Закрыт для среза:** secret-scan + history — 0 blocking; manual items — test/doc примеры | Повторить после финального docs commit; владелец проверяет findings |
| B5 | Desktop smoke текущей beta-сборки | **Закрыт:** actual diagnostic/smoke GUI DOM/bridge/native smoke PASS, pageErrors=0, два viewport; production EXE без remote-debug; screenshots обезличены | Повторять при UI/EXE изменениях |
| B6 | Нет GitHub-hosted CI и draft Release | Workflows готовы; публичный пустой remote настроен | Source push, CI PASS; draft Release проверяется отдельно |
| B7 | Windows 10/LTSC scope | Не проверялись | **Решено:** beta только Windows 11 x64; не расширять без проверки |
| B8 | Разрешение разделено для source и Release | **Source push явно разрешён 2026-11-08**; binary Release не разрешён | Release — отдельное решение после B1/B2/B6 |

Примечание: Windows 10/LTSC не является блокером, если beta явно поддерживает
только Windows 11 x64. Не расширять supported scope без проверки.

## BETA REQUIRED

- [x] Production EXE не регистрирует `--smoke-port`/DevTools; remote debugging
  вынесен в непакетируемую diagnostic-сборку с тегом `smoke`.
- [ ] Повторная production-сборка после финального коммита; SHA-256 и
  `SHA256SUMS.txt` совпадают, версия EXE `0.1.0-beta.1`.
- [ ] Portable ZIP содержит EXE/docs/Known Issues, без debug/test EXE и секретов.
- [ ] Release notes отражают реальные проверки и отсутствие code signing.
- [ ] README screenshots обезличены; hostname/IP/имена не публикуются.
- [ ] Run `scripts/check-release-secrets.ps1 -IncludeHistory`, review findings.
- [x] Owner selected MIT and replaced the placeholder before public source access.
- [ ] GitHub Actions run on disposable runner; draft release assets verified.
- [ ] Visible/manual desktop smoke владельцем (текущий automated DOM/bridge smoke
  не заменяет ручное подтверждение окна и UAC-сценария).

## POST-BETA

- [ ] Real install/update/cancel/clean-install VM matrix, update target version.
- [ ] SMB server/credentials/ACL acceptance; conflict restore.
- [ ] Real reboot/fast startup Verify-only acceptance.
- [ ] Windows 10/LTSC matrix; отдельные builds/WebView2.
- [ ] Drivers: target INF/version/publisher, VM install/reboot.
- [ ] Printers: TCP/IP port creation, driver trust, queue/default/test read-back.
- [ ] Browser/Windows settings: full read-back, restore, GPO coexistence.
- [ ] Elevated worker with original SID or explicit removal of any elevated path;
  session authentication beyond owner/write-ACE.
- [ ] Installer process tree/cancellation; no rollback claims.
- [ ] Code signing certificate, signed release, SmartScreen reputation.
- [ ] Auto-update and offline bundle E2E (только после явного решения).
- [ ] Semantic profile validation (`check_ref`, parameters, dependency closure).
- [ ] Persistent safe diagnostics with error codes/correlation.
- [ ] Frontend lint, accessibility/performance, dependency CVE review.

## FROZEN

Windows 7 Legacy: `legacy/`, `profiles/legacy.yaml`, tests/docs сохранены. Не
добавлять Legacy features/fixes/tests, не включать Legacy в Modern CI/release.
Возврат — только отдельное решение владельца.

## Следующая итерация — 3 задачи

1. **B1**: согласовать и выполнить один безопасный install на Win11 x64 (с
   разрешения владельца); зафиксировать detection/repeat/update/failure/cancel и
   отчёт.
2. **B2**: поднять/получить тестовый SMB-сервер и проверить connect/credentials/
   shortcut/1219 без секретов в отчёте.
3. **B4/B6**: завершить secret-scan, отправить source в настроенный remote и
   проверить CI; draft Release оставить отдельным этапом.

Не начинать новые крупные модули, Legacy или VM-wide roadmap. После задач —
тесты, отдельные коммиты, TASKS/HANDOFF/BETA_READINESS и git status.
