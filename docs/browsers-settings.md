# Браузеры и настройка Windows

Реализованы `core/browsers` (enterprise-политики Chromium/Firefox и ярлыки веб-приложений) и `core/settings` (имя компьютера, часовой пояс, Проводник, электропитание, диагностика Windows Update, статус лицензирования только для чтения). UI — страницы Modern «Браузеры» и «Windows».

## Браузеры

Политики записываются только в поддерживаемые ключи реестра; профили, пароли и базы браузеров не читаются и не изменяются.

- Chrome: `HKCU/HKLM\Software\Policies\Google\Chrome`.
- Edge: `HKCU/HKLM\Software\Policies\Microsoft\Edge`.
- Firefox: `HKCU/HKLM\Software\Policies\Mozilla\Firefox`.

Поддерживается:

- `HomepageLocation` (Chromium) / `Homepage` JSON (Firefox);
- `RestoreOnStartup=4` + `RestoreOnStartupURLs` (Chromium, список через перевод строки);
- `ManagedBookmarks` (Chromium JSON) / `Bookmarks` (Firefox JSON, панель закладок);
- ярлыки веб-приложений `.lnk` для Chrome/Edge: цель — исполняемый файл браузера, аргументы `--app=<URL>`; каталог рабочего стола берётся из Known Folder API.

Область `user` не требует прав; `machine` требует elevated-токен. Dry-run возвращает план изменений без записи. Firefox не поддерживает стартовый список и веб-приложения в этой реализации — это предупреждение, а не тихая имитация. URL проверяются: только http/https, без userinfo, лимиты элементов.

## Настройка Windows

- **Имя компьютера**: валидация DNS-имени, preview, `SetComputerNameExW` (ComputerNamePhysicalDnsHostname), обязательная перезагрузка, требуется администратор.
- **Часовой пояс**: проверка ID через ключ `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Time Zones\<ID>`, чтение `tzutil /g`, применение `tzutil /s <ID>`; неверный ID отклоняется.
- **Проводник**: `HKCU\...\Explorer\Advanced` — Hidden, HideFileExt, ShowSuperHidden, LaunchTo. Preview показывает старые значения; Проводник автоматически не перезапускается.
- **Электропитание**: проверка GUID по `HKLM\SYSTEM\CurrentControlSet\Control\Power\User\PowerSchemes\<GUID>`, чтение `powercfg /getactivescheme`, применение `powercfg /setactive <GUID>`.
- **Windows Update**: read-only диагностика — состояние службы `wuauserv` через SCM, маркеры перезагрузки (WindowsUpdate RebootRequired, CBS RebootPending, PendingFileRenameOperations), наличие политик Windows Update.
- **Лицензирование**: только чтение `SLIsWindowsGenuineLocal` из slc.dll; WinDeploy не активирует, не обходит и не изменяет лицензирование. Рекомендация — штатный `slmgr.vbs /dlv`.

Все изменяющие операции в GUI имеют отдельное подтверждение; сначала доступен dry-run.

## Проверено на текущей Windows

- Go 1.20.14 amd64+386: test/vet/build PASS.
- Логика политик браузеров проверена fake-store тестами (JSON политик, область, elevation, ярлыки); нативные чтения: Desktop, расположение Chrome/Edge/Firefox через App Paths.
- Настройки: реальное чтение имени, часового пояса (`tzutil /g`), схемы электропитания (`powercfg /getactivescheme`), параметров Проводника, состояния службы обновлений, маркеров перезагрузки и `SLIsWindowsGenuineLocal` PASS; dry-run имени/зоны/Проводника/питания не изменяет систему.
- UI: 34 vitest-теста; strict TypeScript/Vite PASS.

## Ограничения

- Запись политик браузеров в реестр не подтверждена в текущей сессии: тестовый процесс запускается с низким обязательным уровнем целостности, и Windows запрещает запись в HKCU\Software. В обычном/повышенном запуске WinDeploy запись выполняется штатными API; это нужно проверить в VM.
- Реальные браузеры не запускались и не проверяли применение политик; профили намеренно не затрагивались.
- Firefox: нет стартовых вкладок и веб-приложений; JSON-политики не проверены реальным Firefox.
- SetComputerNameExW и tzutil/powercfg-изменения не выполнялись (требуют прав и перезагрузки); проверены preview/чтение.
- Диагностика Windows Update не запускает поиск обновлений и не устанавливает их.
