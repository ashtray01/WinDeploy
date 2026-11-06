# WinDeploy — Agent Handoff

## Состояние проекта
Ветка `master`, дерево чистое после финального коммита. Реализованы Stages 1–11: общее Go 1.20-ядро, Modern GUI (Wails v3.1.0/React), Legacy Win32 GUI (Go 1.20/Walk, Win7-совместимый), CLI и release-пакеты. Статус — **preview**, не production и не опубликованный релиз.

Последние коммиты:
- `8559cf8` — тесты CLI выполнения (plan/JSON, --yes, sessions/report).
- `ed3483b` — CI, portable/offline bundles, SHA256SUMS.
- `fc07bb0` — Legacy Windows 7 GUI.
- `b7f7cf4` — выполнение профилей, сессии, отчёты.
- `3068760` — браузеры и настройки Windows.
- `e8c3db3` — драйверы и принтеры.
- `4e2d883` — Modern сеть/папки; `1d38b1a` — core network/shares.

## Что реализовано

### Ядро
- `core/engine` — DAG, precheck/execute/verify, timeout, отмена, opt-in retry, MaxParallel, checkpoint, resume, Windows Installer resource.
- `core/inventory` — system/CPU/RAM/volumes/network/apps, реальные Windows API.
- `core/profiles` — строгая YAML-схема, наследование, выбор компонентов, план.
- `core/packages` — 12 программ, детекция, источники, SHA-256, Authenticode, silent install, повторы, reboot, проверка, кэш, WinGet fallback.
- `core/network` — DNS/DHCP/gateway/TCP/ICMP/HTTP/SMB-445 с таймаутами.
- `core/shares` — WNet, Credential Manager, Error 1219, `.lnk`.
- `core/drivers` — SetupAPI, INF, подпись, pnputil, Win7 fallback/Sysnative.
- `core/printers` — EnumPrinters, default, TCP/IP add, тестовая страница, Kyocera 2460.
- `core/browsers` — Chromium/Firefox политики, закладки, веб-приложения, elevation.
- `core/settings` — имя ПК, часовой пояс, Проводник, электропитание, Windows Update diagnostics, read-only лицензирование.
- `core/deploy` — адаптер компонентов профиля, приватные сессии, resume, `core/reporting` HTML/JSON.
- CLI: `windeploy-inspect`, `windeploy-packages`, `windeploy-run`.

### Modern
- Wails v3.1.0 + React 19/TypeScript/Vite; страницы: Обзор, Профили, Установка ПО, Выполнение, Сеть, Сетевые папки, Драйверы, Принтеры, Браузеры, Windows, Журнал, Inventory.
- 37 vitest-тестов, strict typecheck, production Vite, Go service tests/vet; 5 CLI-тестов `windeploy-run`.
- Сборка: `scripts/build-modern.ps1`; EXE `bin/WinDeploy-Windows-x64.exe`.

### Legacy
- Отдельный `legacy/` Go 1.20-модуль, Walk/Win32, без WebView2/.NET.
- Сводка, установка программ, отмена, диагностика сети, UNC-папки, read-only update/licensing.
- Манифест Common Controls 6 (`rsrc_windows_*.syso`) обязателен: без него Walk падал на `TTM_ADDTOOL`.
- `scripts/build-legacy.ps1` собирает x64/x86, запускает тесты/`--selftest`/опционально `--smoke`.

### Release
- `scripts/package-release.ps1`: portable x64/x86, offline x64 + `catalog` + проверенные установщики (7-Zip, SumatraPDF, PrinterOne), SHA256SUMS, release notes.
- `.github/workflows/ci.yml` и `release.yml`; CI не выполнялся на GitHub (remote отсутствует), публикация не сделана.

## Проверки (реальные)
- Go 1.20.14 `windows/amd64` и `windows/386`: финальный `scripts/test-core.ps1` — test/vet/build PASS.
- Modern: финальный `scripts/build-modern.ps1` PASS; 37 UI-тестов; Wails EXE собран; `go mod verify` PASS.
- Legacy: test/vet x64/x86 PASS; `--selftest` PASS; реальное окно `--smoke` PASS на Windows 11 x64 и x86 (WOW64), auto-close 3 с.
- Реальная read-only сессия `windeploy-run` (diagnostics DNS localhost) создала session JSON + HTML/JSON отчёты, exit 0; resume/отчёт проверены тестами.
- GitHub Actions workflows: YAML parse PASS; `actionlint v1.7.12` — 0 ошибок (локально, без запуска на GitHub).
- Ранние реальные проверки: 212 устройств/210 версий драйверов, 4 принтера, HTTPS-загрузки 7-Zip/SumatraPDF/PrinterOne с совпавшими SHA-256, Credential Manager, `.lnk`, сеть.
- Preview-артефакты: `dist/WinDeploy-0.1.0-preview.1-{portable-x64,portable-x86,offline-x64}.zip`, `dist/SHA256SUMS.txt` (актуальные хеши — в файле; после последней перепаковки: portable-x64 `0794b47b…`, portable-x86 `18268319…`, offline-x64 `00b0f504…`).

## Ограничения и блокеры
- **Modern GUI smoke** на текущем хосте невозможен: WebView2 controller `0x8000FFFF` даже для минимального Wails. EXE собирается; startup-лог `%LOCALAPPDATA%\WinDeploy\logs\startup.log`.
- **Windows 7 SP1 VM отсутствует.** Legacy проверен только на Windows 11; Go 1.20/Walk не доказывают Win7 runtime.
- **Реальные изменяющие операции** (установка ПО/драйверов/принтеров, SMB, политики, настройки, переименование, reboot continuation) не выполнялись: нет изолированной VM/прав.
- Registry writes из тестового процесса блокируются низким integrity (HKCU\Software), поэтому политики браузеров проверены fake-store, не реальной записью.
- WinGet в песочнице не получает identity; проверен HTTPS-путь.
- GitHub remote/CLI отсутствуют; CI не запускался; **v0.1.0 pre-release не опубликован и тег не создан** (критерии в `docs/release.md` не выполнены).
- Race detector недоступен (нет C compiler).

## Инструкция следующему агенту
1. `git status`, `git log -8 --oneline`, прочитать этот файл, `TASKS.md`, `docs/release.md`; не переисследовать готовые модули.
2. Core: `.\scripts\test-core.ps1 -Go (Join-Path $PWD 'local/_toolchains/go1.20.14/go/bin/go.exe') -BuildCache go-build-go120`.
3. Modern: `.\scripts\build-modern.ps1`; UI: `npm run typecheck`/`npm test` в `desktop/modern/frontend`.
4. Legacy: `.\scripts\build-legacy.ps1 -GuiSmoke`; отдельно `legacy\go test ./...`.
5. Release: `.\scripts\package-release.ps1 -Version 0.1.0-preview.1`; не публиковать без VM-матрицы и Modern smoke.
6. Не коммитить `bin/`, `dist/`, `cache/`, `local/`, логи и приватные данные; не менять глобальную Git identity.

## Следующая конкретная задача
Поднять изолированную Windows 7 SP1 x86 VM (чистая, без обновлений): запустить `WinDeploy-Legacy-Windows-x86.exe --selftest` и `--smoke`, затем установить 7-Zip из offline-набора с проверкой SHA-256/подписи и подтвердить работу окна/журнала. Результат зафиксировать в `docs/legacy.md`/HANDOFF; только после этого переходить к Win7 x64/Windows 10/11 VM и Modern WebView2 smoke.

## Чего нельзя делать
Не выполнять установки/драйверы/принтеры/SMB/настройки на рабочем хосте вне авторизованной VM. Не отключать SHA-256/подпись, UAC/Defender/Firewall. Не публиковать релиз как production-ready. Не коммитить секреты, EXE, кэши, логи, hostname/IP/MAC и списки приложений.
