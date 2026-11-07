# Contributing to WinDeploy

WinDeploy находится в public beta. Приоритет — безопасный основной сценарий и
честные статусы, а не количество новых функций. Перед крупной работой откройте
Issue и опишите сценарий, риск и способ проверки.

## Что не принимается

- новые функции Legacy/Windows 7: Legacy FROZEN;
- обход SID/session/ACL gate, «удобное» повышение прав или impersonation;
- фиктивный `Success`, скрытые системные изменения, секреты в логах/отчётах;
- отключение или переписывание failing tests ради зелёного CI;
- реальные установки драйверов/принтеров/SMB и изменения политик на рабочей
  машине без отдельного разрешения владельца.

## Требования

- Go: root-модуль остаётся `go 1.20`; Modern — отдельный модуль `desktop/modern`
  с Go 1.26.x. Не повышайте root Go и не добавляйте Modern-зависимости в root.
- Node.js 24.x, npm 11.x; версии frontend закреплены lockfile.
- Wails CLI v3.1.0. Установка описана в [docs/modern.md](docs/modern.md).
- Windows 11 x64 — основная платформа. Windows 10/LTSC не заявляются без
  отдельной проверки.

## Сборка и проверки

```powershell
# Root: Go 1.20, тесты/vet/сборка amd64+386
./scripts/test-core.ps1 -Go <path-to-go1.20.14> -BuildCache ci-go120
# Только на одноразовом runner можно разрешить Credential Manager/registry tests:
./scripts/test-core.ps1 -Go <path-to-go1.20.14> -AllowNativeWrites

# Modern: bindings, 44 UI tests, strict TypeScript/Vite, Go tests/vet, Wails EXE
./scripts/build-modern.ps1 -Diagnostic

# Согласованность версии и release-артефакты
./scripts/verify-version.ps1
./scripts/package-modern-release.ps1 -SkipBuild
```

`scripts/test-core.ps1` создаёт временные файлы в `cache/tmp` и восстанавливает
окружение. Если файловая песочница запрещает смену DACL, ACL-тесты не считаются
пройденными: запускайте проверку в обычной оболочке/CI, не отключайте тесты.
OneDrive-индексатор может кратковременно блокировать rename; state/session stores
используют ограниченные повторы, а не бесконечный retry.

Для frontend отдельно:

```powershell
cd desktop/modern/frontend
npm ci
npm run typecheck
npm test
npm run build
```

Lint-конфигурации пока нет; typecheck не заменяет lint.

## Definition of done

Для изменения нужны:

1. критерий готовности и негативный тест для security/ошибочного пути;
2. `gofmt`, релевантные test/vet/build; для UI — typecheck, tests, production build;
3. отсутствие секретов, EXE, логов и персональных данных в diff;
4. обновление `TASKS.md`, а после логического пакета — `HANDOFF.md`;
5. отдельный осмысленный commit (`feat`, `fix`, `test`, `docs`, `chore`).

Не используйте force push/reset hard и не меняйте глобальную Git identity.

## Коммиты и ветки

- Одна итерация — 2–4 логически завершённые задачи.
- Работайте в отдельной ветке, если несколько агентов/людей меняют репозиторий.
- Сообщение коммита: краткий императив и компонент, например
  `fix(security): reject untrusted session storage`.
- Pull request должен содержать реальные результаты проверок и честно перечислять
  непроверенные сценарии.

## Лицензия

Лицензия ещё не выбрана. Отправка PR не означает согласие на какую-либо лицензию;
владелец должен принять решение до публичного открытия исходников. См.
[LICENSE](LICENSE).
