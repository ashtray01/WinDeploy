# Публикация WinDeploy beta — чеклист владельца

Этот чеклист не даёт разрешения на публикацию. Публичный репозиторий и Release
создаются только после явного подтверждения владельца.

## 1. До открытия исходников

- [ ] Выбрана лицензия и заменён `LICENSE` (сейчас осознанная заглушка).
- [ ] Зафиксирован публичный owner/repository URL; заменены ссылки Releases/Issues
      в `README.md` и release notes.
- [ ] Владелец проверил, что в репозитории нет приватных конфигураций, внутренних
      URL, имён коллег/клиентов, inventory и скриншотов с персональными данными.
- [ ] Выполнен `scripts/check-release-secrets.ps1`; все находки разобраны.
- [ ] При находке секрета: репозиторий не публикуется, ключ ротирован, история
      очищена только согласованным способом. Force-push без разрешения запрещён.
- [ ] Проверено, что `temp/windeploy-icon.png`, локальные каталоги, EXE, ZIP,
      `cache/`, `bin/`, `dist/` и логи не попадут в коммит.
- [ ] Решено, будет ли beta распространяться как binary-only или с исходниками.
      `LICENSE` должен соответствовать этому решению.

## 2. Функциональные гейты

- [ ] B1: один согласованный безопасный пакет установлен на Windows 11 x64,
      detection/повтор/update/failure/cancel подтверждены, отчёт совпадает.
- [ ] B2: SMB-подключение и обязательный ярлык проверены с тестовым сервером;
      в отчёте нет пароля; Error 1219 обрабатывается только после подтверждения.
- [ ] B5: `scripts/build-modern.ps1 -Diagnostic` PASS.
- [ ] Visible desktop smoke текущей beta-сборки: основной маршрут, прогресс,
      ошибки, журнал, два viewport. Результат записан в HANDOFF.
- [ ] Windows 10/LTSC либо проверены, либо явно исключены из supported scope.
- [ ] Драйверы/принтеры/политики/Windows settings либо проверены, либо остаются
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

- [ ] EXE version resource = `0.1.0-beta.1`, имя `WinDeploy`.
- [ ] EXE воспроизводится из чистого checkout с закреплёнными toolchain/deps;
      два последовательных build дают одинаковый SHA-256 (или различие объяснено
      документированной метадатой).
- [ ] Production frontend assets встроены; EXE не запускает Vite/dev server.
- [ ] `SHA256SUMS.txt` соответствует фактическим файлам.
- [ ] Code signing отсутствует — это написано в release notes/Known Issues.
      Не заявлять наличие подписи без сертификата.

## 4. GitHub

- [ ] Remote создан только с разрешения владельца; backup/теги не переписаны.
- [ ] Modern CI (`ci.yml`) PASS на GitHub-hosted runner.
- [ ] Legacy CI не требуется для beta release.
- [ ] Release workflow запускается по тегу `v0.1.0-beta.1`; tag совпадает с
      `VERSION`.
- [ ] Workflow создаёт **draft** pre-release, не публикует автоматически.
- [ ] Проверены Assets: `WinDeploy-Windows-x64.exe`, `SHA256SUMS.txt`,
      portable ZIP, release notes.
- [ ] Владелец вручную проверил draft и явно нажал Publish. Только после этого
      ссылки в README можно считать публичными.

## 5. После публикации

- [ ] Проверить доступность Release и Issues без авторизации.
- [ ] Добавить реальные ссылки Releases/Issues в README и release notes.
- [ ] Обновить `docs/BETA_READINESS.md` и `HANDOFF.md` фактическим URL/датой.
- [ ] Отслеживать Issues с шаблонами; не обещать stable-совместимость.
