# WinDeploy — controlled development (Public Beta)

## Текущий этап

2026-11-08: [v0.1.0-beta.1](https://github.com/ashtray01/WinDeploy/releases/tag/v0.1.0-beta.1)
опубликован как **GitHub Release (Latest)**, не Stable. Release commit:
`56fe16351720b4a74a5e773a7a26e270fe4bbf84`. Windows 11 x64 — проверенная платформа;
Win10/LTSC не подтверждены, Windows 7 Legacy FROZEN и исключён из Modern release.
Владелец явно разрешил выпуск после проверки EXE; B1/B2 честно раскрыты, но не
выполнены.

## Сделано в этой итерации — 3 задачи

- [x] Проверены текущие Git/remote/VERSION/tags/Releases (до выпуска прежних
      тегов/Releases не было), MIT и существующий CI/CD.
      Добавлен отсутствовавший history secret preflight (`fb2920c`), исправлены
      устаревшие packaged docs (`56fe163`); runtime/UI/Legacy не менялись.
- [x] Modern CI `42421280314` и release workflow `42421767518` PASS на release
      commit. Annotated tag создан без подписи (не настроена). Draft проверен,
      assets скачаны и сверены до публикации; Release опубликован и переведён в
      Latest после проверки EXE владельцем.
- [x] Публичная HTML/API страница и все 4 downloads проверены без авторизации;
      SHA-256 совпадают с draft, manifest и GitHub digest. Итоговые документы
      обновлены отдельным коммитом без изменения release tag/assets.

## BETA BLOCKER / открытые ограничения

| ID | Статус | Доказательство / следующий шаг |
|---|---|---|
| B1 | Открытый непроверенный beta-сценарий | Нет real package-install/update/cancel acceptance. По явному решению владельца не общий запрет выпуска; нужен согласованный пакет и VM/разрешение |
| B2 | Открытый непроверенный beta-сценарий | Нет real SMB server/credentials/1219 acceptance; нужен тестовый сервер/учётная запись |
| B3 | Закрыт | MIT, ashtray01/WinDeploy |
| B4 | Закрыт для release-среза | Локальный и release CI history preflight: 0 blocking, manual items test/docs |
| B5 | Исторический Windows 11 actual GUI smoke PASS | Runtime/UI в этой итерации не менялись; скачанный EXE заново не запускался |
| B6 | Закрыт | CI/release PASS, draft/assets/public downloads проверены |
| B7 | Scope ограничен | Windows 11 x64; Win10/LTSC не подтверждены |
| B8 | Закрыт | Явное разрешение владельца на публичный v0.1.0-beta.1 Release после проверки EXE |

B1/B2 не объявлены PASS и не перенесены в POST-BETA как решённые security-проблемы.
Runtime SID/session/ACL/installer gates сохранены. Полный security audit не выполнялся.

## BETA REQUIRED

- [x] VERSION/tag/EXE совпадают; EXE и ZIP BUILD-INFO содержат release commit.
- [x] Core Go 1.20 amd64+386 test/vet/build, Modern tests/vet, frontend
      tests/strict typecheck/build и Wails production EXE PASS в CI.
- [x] Portable ZIP, MIT, Release Notes, SHA256SUMS и все 4 публичных downloads
      проверены; EXE NotSigned, production debug flags отсутствуют.
- [x] Release Notes раскрывают beta, Win11 scope, Win10/LTSC/Win7 ограничения,
      отсутствие подписи, экспериментальные функции и невыполненные B1/B2.
- [x] Draft проверен, опубликован как Release и переведён в Latest по разрешению
      владельца после проверки EXE.
- [ ] Новый visible/manual desktop smoke опубликованного EXE не выполнялся;
      предыдущий actual diagnostic DOM/bridge smoke не заменяет его.

Подробные результаты и SHA-256: [RELEASE_VERIFICATION](docs/RELEASE_VERIFICATION_v0.1.0-beta.1.md).

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

## Следующая итерация — v0.1.0-beta.2 (3 задачи)

1. B1: реальный package install/update/repeat/failure/cancel и отчёт на
   одноразовой Windows 11 VM либо с отдельного разрешения владельца.
2. B2: реальный SMB server/credentials/shortcut/1219 acceptance без секретов в logs.
3. Исправления по воспроизводимым GitHub Issues; следующий релиз выпускать
   отдельной задачей. Не переписывать v0.1.0-beta.1 и не начинать Legacy roadmap.
