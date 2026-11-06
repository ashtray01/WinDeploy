# WinDeploy — Agent Handoff

## Состояние проекта
Ветка master. Завершены Stage 5 «Package Manager» и Stage 6 «Network & Shares» (ядро + Modern GUI + CLI). Последние коммиты: 29f41fd, 6af236b, 1d38b1a, 4e2d883. Следующий этап — Stage 7 «Drivers & Printers». Root остаётся Go 1.20; Modern — отдельный модуль Go 1.26/Wails 3.1.

## Что реализовано

### Package Manager (Stage 5)
- `core/packages`: строгий каталог (12 программ), детекция (Uninstall, MSI ProductCode, файлы/маски), сравнение версий, выбор источника (WinGet/HTTPS/локальный файл) с приоритетами, потоковая загрузка с SHA-256 и проверенным кэшем, Authenticode через WinVerifyTrust + извлечение субъекта сертификата, тихая установка EXE/MSI/portable, классификация кодов выхода, повтор при 1618/retry-кодах, `RebootRequired`, пост-проверка, запрет переустановки и синхронная проверка прав администратора.
- `catalog/packages.yaml`: 7-Zip, Chrome, Firefox, Яндекс Браузер, Telegram, TG WS Proxy, MAX, VLC, MPC-HC, SumatraPDF, IrfanView, PrinterOne. WinGet-ID проверены на месте; HTTPS-URL и SHA-256 взяты из актуальных WinGet-манифестов; есть офлайн-файлы `%ProgramData%\WinDeploy\Packages`.
- Modern: страница «Установка ПО» — галочки, состояние/версия/источник/интеграция, установка в фоне, прогресс, журнал, отмена, итоговая таблица; без прав администратора системные пакеты блокируются.
- CLI `cmd/windeploy-packages`: `list`, `plan`, `download`, `install` (требует `--yes`), `--dry-run`, `--no-winget`, JSON.
- WinGet discovery: App Execution Alias не открывается `os.Stat`; ищем AppX-путь и используем fallback через `cmd.exe`/PowerShell. В текущей песочнице WinGet не получает identity, поэтому план честно уходит на HTTPS-источники.

### Network & Shares (Stage 6)
- `core/network`: DNS (системный и конкретный сервер), DHCP-сводка, шлюз, TCP, ICMP, HTTP, SMB-445; таймауты на проверку, статусы ok/failed/timeout/skipped/unsupported, отмена, безопасные детали.
- `core/shares`: валидация UNC, WNetAddConnection2W, Windows Credential Manager (generic DPAPI, таргет `WinDeploy:share:<server>`), обнаружение Error 1219 и разрешение только по явному подтверждению, перечисление подключений WNet, нативные ярлыки `.lnk` (IShellLink/IPersistFile), Known Folder Desktop.
- Modern: страницы «Диагностика» и «Сетевые папки» (dry-run, подтверждение, конфликт 1219, список подключений).
- CLI `windeploy-inspect network` для реальной диагностики.

### Прочее
- `docs/packages.md`, `docs/network.md`, обновлены `docs/architecture.md`, `docs/modern.md`, `TASKS.md`.
- Startup-диагностика Modern: журнал `%LOCALAPPDATA%\WinDeploy\logs\startup.log`, проверка WebView2 Runtime до Wails, нативные сообщения с путём лога.

## Проверки
- Go 1.20.14 (windows/amd64 и windows/386): `scripts/test-core.ps1` — test/vet/build PASS после Stage 5 и Stage 6. Включая native-тесты Windows: подпись Authenticode (подписанный вендорский файл PASS, неподписанный отклонён), чтение версии файла, exit code/отмена процесса, Credential Manager round-trip, WNet enumeration, создание .lnk.
- Modern Go: `go test ./...`, `go vet ./...` PASS; `scripts/build-modern.ps1` PASS (bindings, 23 UI tests, strict TypeScript, Vite production, module verify, Windows EXE 18+ МБ).
- UI: 23 vitest-теста (Dashboard, Inventory, Profiles, Packages, Network, Shares, Shell, Journal, ErrorBoundary) PASS.
- Реальная Windows (Windows 11 x64 build 26200): детекция установленных 7-Zip/Chrome/Firefox/Telegram/VLC/SumatraPDF/IrfanView; HTTPS-загрузки 7-Zip 26.05 (1 663 506 B, SHA-256 `fd99e86f…`), SumatraPDF 3.6.1 (11 075 960 B, `1eee71cc…`), PrinterOne 1.0.1 (21 647 518 B, `d0211d52…`) — хеши совпали; повторная загрузка из кэша без сети; сеть: DNS localhost, прямой DNS-сервер 10.20.20.1, TCP 445, HTTP локального сервера, DHCP-статусы, ICMP честно недоступен.
- Реальный GUI smoke **не выполнялся** в этой сессии: хост перестал создавать WebView2-контроллер (`0x8000FFFF`) для любого Wails-приложения, включая минимальный тест и сборку из предыдущего коммита. Это внешняя среда; startup-диагностика теперь фиксирует такие сбои в файле. Предыдущий реальный smoke был PASS на сборке 169c239.

## Ограничения
- Реальная установка стороннего ПО и подключение к настоящему SMB-серверу не выполнялись: нет прав администратора и изолированной VM/сервера. WinGet-путь не проверен в нормальной интерактивной сессии.
- Windows 7 SP1 VM отсутствует; Legacy GUI ещё не реализован. Go 1.20 на текущей Windows не доказывает Win7 runtime.
- Windows 10 VM, отсутствующий/старый WebView2, DPI/мультимонитор, ARM не проверены. Race detector недоступен (нет C compiler).
- HTTPS-хеши закреплены на версии WinGet-манифеста; при обновлении неизменяемого URL будет fail-closed.
- Shares не интегрированы в исполнение профилей (Task Engine) — это Stage 9; ярлыки/автозапуск portable-программ не создаются; удаление пакетов не реализовано.
- GitHub remote/CI/release отсутствуют.

## Коммиты сессии
- 29f41fd — Package Manager: download API, CLI, WinGet discovery.
- 6af236b — Modern: страница пакетов, service, startup-диагностика.
- 1d38b1a — core/network + core/shares + network CLI.
- 4e2d883 — Modern: страницы сети/папок, документация.
Предыдущая база: 169c239 (Modern 10).

## Архитектурные решения
- Каталог и установка не зависят от UI; все Windows-эффекты в адаптерах, Manager тестируется fake-адаптерами.
- Integrity по умолчанию — pinned SHA-256; подпись издателя обязательна только при `integrity: signature` или заданном `signer`. `winget` использует собственные манифест-хеши.
- Credential Manager: generic DPAPI credential, так как domain-password тип не сохраняет произвольный blob вне домена; пароль никогда не пишется в профиль/лог/командную строку.
- Error 1219 никогда не разрешается автоматически: сначала typed error, затем повторный явный запрос.
- `core/network` использует `ping.exe` только для ICMP; TCP/DNS/HTTP — нативные Go API.
- `core/shares` COM-вызовы реализованы вручную через vtable; это проверено реальным созданием `.lnk`.

## Следующая конкретная задача
Stage 7 — Drivers & Printers: `core/drivers` (unknown devices, Hardware ID, installed drivers, INF install with signature check and Windows-version/arch matching, local/network driver catalog, Windows Update-compatible mechanism) и `core/printers` (TCP/IP ports, shared Windows printers, default printer, test page, Kyocera 2460 с обязательным уточнением модели), затем Modern-страницы и CLI. Изменяющие операции — только с правами и явным подтверждением; реальная проверка — в VM.

## Инструкция следующему агенту
1. `git status`, `git log -5 --oneline`, прочитать этот файл, `TASKS.md`, `docs/packages.md`, `docs/network.md`; не переисследовать готовое ядро.
2. Core-проверка: `.\scripts\test-core.ps1 -Go (Join-Path $PWD 'local/_toolchains/go1.20.14/go/bin/go.exe') -BuildCache go-build-go120`. Скрипт сам переносит TEMP внутрь workspace из-за песочницы.
3. Modern: `.\scripts\build-modern.ps1`; UI отдельно `npm test` и `npm run typecheck` в `desktop/modern/frontend`.
4. Реальный CLI: `bin\WinDeploy-Packages-Windows-x64.exe plan|download -no-winget -cache local/pkg-cache`, `bin\WinDeploy-Inspect-Windows-x64.exe network --dns localhost --tcp 127.0.0.1:445`.
5. GUI smoke: `node scripts/gui-modern-smoke.mjs` только если WebView2 снова создаёт контроллер; перед запуском проверьте `%LOCALAPPDATA%\WinDeploy\logs\startup.log`. Не выдавайте сборку за desktop smoke.
6. Git commit разрешён без escalation в текущем режиме; не коммитить `bin/`, `cache/`, `local/`, логи и приватные данные. Перед завершением обновить TASKS/HANDOFF и проверить `git status`.

## Чего нельзя делать
Не устанавливать пакеты/драйверы/принтеры и не менять сеть/защиту/Windows settings на рабочем хосте; изменяющие операции только в отдельно авторизованной VM. Не коммитить EXE, кэши, логи, hostname/IP/MAC/списки приложений и пароли. Не отключать проверку SHA-256/подписи ради «удобства». Не публиковать релиз как production-ready без реальных проверок.
