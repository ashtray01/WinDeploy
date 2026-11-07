# Changelog

Все значимые изменения WinDeploy. Формат близок к Keep a Changelog, версии —
SemVer. Проект находится в стадии public beta.

## [0.1.0-beta.1] — 2026-11-08

Опубликован [GitHub Release (Latest, Public Beta)](https://github.com/ashtray01/WinDeploy/releases/tag/v0.1.0-beta.1),
commit `56fe16351720b4a74a5e773a7a26e270fe4bbf84`.
[Release workflow](https://github.com/ashtray01/WinDeploy/actions/runs/42421767518) PASS;
публичные downloads и SHA-256 проверены. [Доказательства и хеши](docs/RELEASE_VERIFICATION_v0.1.0-beta.1.md).
Документация после выпуска обновлена отдельно; tag/assets не переписывались.

### Added

- Первая Modern-only сборка `WinDeploy-Windows-x64.exe` с единым номером версии
  в UI, Windows version resource и release-артефактах.
- Экран выполнения требует построения предварительного плана и явного
  подтверждения до запуска профиля.
- Установка ПО требует явного выбора программ и отдельного подтверждения;
  каталог больше не выбирает устанавливаемые пакеты автоматически.
- SHA-256 и portable-архив для beta-релиза, Modern-only CI и workflow, который
  создаёт только **draft** release после проверки тега; публикация/повышение —
  отдельное явное действие владельца.
- Публичные документы: Known Issues, Security, Contributing, BETA_READINESS,
  release notes, issue templates.
- Ограниченные повторы атомарной замены файлов для OneDrive/antivirus-конфликтов
  при сохранении checkpoint/session.

### Changed

- Единый источник версии — `VERSION`; frontend, EXE, UI и release workflow
  проверяются на согласованность.
- Modern CI больше не зависит от Legacy. Legacy остаётся FROZEN и запускается
  только вручную.
- Основной пользовательский маршрут явно сообщает, что часть функций
  экспериментальна, а при потере связи с backend показывает устаревший прогресс.

### Fixed

- Устранена ложная зелёная плашка «Завершено» при ошибке установки пакета.
- Startup log production EXE больше не остаётся пустым, когда GUI-процесс не имеет
  валидного stderr: запись в файл выполняется первой, stderr остаётся best-effort.
- Production EXE больше не регистрирует `--smoke-port` и не включает WebView2
  DevTools; remote debugging вынесен в непакетируемую diagnostic-сборку.
- Если основной каталог WebView2 недоступен, приложение проверяет
  `%LOCALAPPDATA%\WinDeploy\WebView2-fallback`, `%TEMP%\WinDeploy\WebView2` и
  портативный `WinDeploy-WebView2Data` рядом с EXE. Автоматический fallback
  требует owner/ACL текущего пользователя, отсутствия символических ссылок и
  успешной записи; Program Files не используется. При переходе показывается
  нативное окно «Продолжить с резервным каталогом» / «Выход» без UAC. Выбор
  запоминается, повторный запуск использует тот же каталог. Существующие данные и
  ACL не изменяются.
- Прогресс установки больше не показывает 100% для незавершённой или пустой
  операции.
- Ошибки чтения сохранённых сессий и потери polling-связи больше не скрываются.
- `go test` использует контролируемую временную директорию; ACL-тесты
  воспроизводимо выполняются при разрешённой смене DACL только во временном
  каталоге.
- Session files: bounded read, strict JSON, identity binding и owner/ACL gate
  перед изменяющим resume (входит в beta).

### Security

- Release workflow проверяет tracked files и Git history существующим secret preflight до сборки.

- SID пользователя процесса сверяется с владельцем интерактивного shell перед
  user-scoped изменениями.
- Session storage проверяется по owner и write-ACE; повреждённые и слишком большие
  файлы отклоняются.
- Установщики проверяются по закреплённому SHA-256 и/или Authenticode-подписи;
  источник с обязательной подписью fail closed.

### Known limitations

- EXE не подписан сертификатом code signing.
- Windows 10/LTSC не проверены; beta заявляет только Windows 11 x64.
- Реальная установка согласованного пакета, SMB-сервер и reboot acceptance ещё не
  закрыты на чистой VM.
- Драйверы, принтеры, политики браузеров и изменение параметров Windows —
  экспериментальные, без VM-приёмки.
- Windows 7 Legacy FROZEN и в beta-артефакты не входит.
