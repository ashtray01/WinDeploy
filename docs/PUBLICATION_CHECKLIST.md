# Публикация WinDeploy beta — чеклист владельца

Владелец разрешил первый source push в публичный репозиторий 2026-11-08 и выбрал
MIT. В текущей задаче владелец также явно разрешил v0.1.0-beta.1 Pre-release:
после CI PASS, проверки draft и SHA-256 скачанных assets. B1/B2 остаются непроверенными beta-сценариями, не общим запретом публикации.

Статус 2026-11-08: Pre-release опубликован; [результаты и хеши](RELEASE_VERIFICATION_v0.1.0-beta.1.md).
Невыполненные функциональные/ручные проверки ниже остаются открытыми.

## 1. До открытия исходников

- [x] Владелец выбрал MIT; `LICENSE` заменён до первого source push.
- [x] Зафиксирован публичный owner/repository URL
      `https://github.com/ashtray01/WinDeploy`; ссылки README заменены.
- [ ] Владелец проверил, что в репозитории нет приватных конфигураций, внутренних
      URL, имён коллег/клиентов, inventory и скриншотов с персональными данными.
- [x] Выполнен `scripts/check-release-secrets.ps1 -IncludeHistory`; blocking
      findings 0, manual-review findings — только ранее описанные test/docs примеры.
- [ ] При находке секрета: репозиторий не публикуется, ключ ротирован, история
      очищена только согласованным способом. Force-push без разрешения запрещён.
- [ ] Проверено, что `temp/windeploy-icon.png`, локальные каталоги, EXE, ZIP,
      `cache/`, `bin/`, `dist/` и логи не попадут в коммит.
- [ ] Решено, будет ли beta распространяться как binary-only или с исходниками.
      `LICENSE` должен соответствовать этому решению.

## 2. Функциональные проверки и открытые ограничения

- [ ] B1: один согласованный безопасный пакет установлен на Windows 11 x64,
      detection/повтор/update/failure/cancel подтверждены, отчёт совпадает.
- [ ] B2: SMB-подключение и обязательный ярлык проверены с тестовым сервером;
      в отчёте нет пароля; Error 1219 обрабатывается только после подтверждения.
- [ ] B5: `scripts/build-modern.ps1 -Diagnostic` PASS.
- [ ] Visible desktop smoke текущей beta-сборки: основной маршрут, прогресс,
      ошибки, журнал, два viewport. Результат записан в HANDOFF.
- [x] Windows 10/LTSC либо проверены, либо явно исключены из supported scope.
- [x] Драйверы/принтеры/политики/Windows settings либо проверены, либо остаются
      явно экспериментальными в UI, README и release notes.

## 3. Локальные проверки

```powershell
# Root Go 1.20
./scripts/test-core.ps1 -Go <path-to-go1.20.14> -BuildCache ci-go120

# Modern
./scripts/build-modern.ps1 -Diagnostic
./scripts/verify-version.ps1
./scripts/package-modern-release.ps1 -SkipBuild

# Проверить фактические хеши
Get-Content dist/SHA256SUMS.txt
Get-FileHash dist/WinDeploy-Windows-x64.exe -Algorithm SHA256
Get-FileHash dist/WinDeploy-0.1.0-beta.1-portable-x64.zip -Algorithm SHA256
Get-AuthenticodeSignature dist/WinDeploy-Windows-x64.exe
```

- [x] EXE version resource = `0.1.0-beta.1`, имя `WinDeploy`.
- [ ] EXE воспроизводится из чистого checkout с закреплёнными toolchain/deps;
      два последовательных build дают одинаковый SHA-256 (или различие объяснено
      документированной метадатой).
- [x] Production frontend assets встроены; EXE не запускает Vite/dev server.
- [x] `SHA256SUMS.txt` соответствует фактическим файлам.
- [x] Code signing отсутствует — это написано в release notes/Known Issues.
      Не заявлять наличие подписи без сертификата.

## 4. GitHub

- [x] Remote создан только с разрешения владельца; `origin` указывает на
      `https://github.com/ashtray01/WinDeploy.git`; backup/теги не переписаны.
- [x] Modern CI (`ci.yml`) PASS на GitHub-hosted runner: run `42421280314` на release commit.
- [x] Legacy CI не требуется для beta release.
- [x] Release workflow запускается по тегу `v0.1.0-beta.1`; tag совпадает с
      `VERSION`.
- [x] Workflow создаёт **draft** pre-release, не публикует автоматически.
- [x] Проверены Assets: `WinDeploy-Windows-x64.exe`, `SHA256SUMS.txt`,
      portable ZIP, release notes.
- [x] Draft проверен и опубликован в рамках явного разрешения владельца. После этого
      ссылки в README можно считать публичными.

## 5. После публикации

- [x] Проверить доступность Release и Issues без авторизации.
- [x] Добавить реальные ссылки Releases/Issues в README и release notes.
- [x] Обновить `docs/BETA_READINESS.md` и `HANDOFF.md` фактическим URL/датой.
- [ ] Отслеживать Issues с шаблонами; не обещать stable-совместимость.
