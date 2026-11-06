# Read-only Inventory и CLI

`inventory.Collect(ctx, inventory.Native{})` собирает семь независимых секций. Ошибка одной секции сохраняет остальные. У каждой секции status: ok, partial, failed, cancelled или not-collected. Полный успешный снимок имеет Complete=true и err=nil; частичный — Complete=false и ErrIncomplete; отмена возвращает context error и оставшиеся not-collected. Сырые ошибки провайдера не попадают в JSON.

Данные: hostname, Windows version/build/SP/product type, native/process architecture; первая CPU brand/vendor и число логических процессоров across groups; usable physical RAM; logical volumes; интерфейсы IPv4/IPv6; gateways/DNS/DHCPv4/link speeds; видимые Uninstall entries HKLM/HKCU. Provider — набор маленьких интерфейсов, его можно подменить для unit-тестов или UI.

Источники и границы:

- [RtlGetVersion](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/nf-wdm-rtlgetversion) через x/sys: исключает обычную зависимость GetVersionEx от manifest; application compatibility layer всё ещё может влиять на ответ. В текущем smoke-test build дополнительно совпал с CurrentBuildNumber в реестре. Windows Server, Win7 RTM и неизвестные версии не получают поддерживаемый workstation token.
- [GetNativeSystemInfo](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-getnativesysteminfo): native OS architecture отдельно от runtime.GOARCH. Проверено: x86 EXE на текущей x64 Windows сообщает native=amd64, process=386. Поведение ARM-emulation отличается; arm64 не входит в поддерживаемую матрицу выполнения.
- CPU: read-only HARDWARE registry + [GetActiveProcessorCount](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-getactiveprocessorcount) для всех групп. Это не инвентаризация каждой физической CPU/socket или гибридной топологии.
- RAM: [GlobalMemoryStatusEx](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-globalmemorystatusex), 64-bit physical counters; это доступная ОС RAM, не сумма паспортной ёмкости DIMM.
- Volumes: GetLogicalDriveStrings/GetDriveType и [GetDiskFreeSpaceEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdiskfreespaceexw). Total/available могут зависеть от quota вызывающего пользователя. Fixed/ramdisk capacity читается; network/removable/optical остаются capacity_known=false с явной причиной. Физические диски, разделы, модели и SMART пока не читаются.
- Interfaces: net.Interfaces/Addrs, локальные flags/MTU/MAC/prefixes. Up не доказывает интернет/доступность сервера. [GetAdaptersAddresses](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getadaptersaddresses) добавляет gateway/DNS/link speed и IPv4/DHCPv4 flags; link-local IPv6 сохраняет scope. DHCPv6 lease, маршруты, Wi-Fi SSID, ICMP/TCP/SMB проверки относятся к этапу Network.
- Apps: [Uninstall registry metadata](https://learn.microsoft.com/en-us/windows/win32/msi/uninstall-registry-key), HKLM и HKCU эффективного пользователя процесса, [обе registry views](https://learn.microsoft.com/en-us/windows/win32/winprog64/accessing-an-alternate-registry-view) на x64. Shared записи объединяются только при совпадении key/scope/name/version/publisher/installer flag. Без UninstallString, InstallLocation, Win32_Product или исполнения команд. HKCU при elevation — пользователь процесса; интерактивный пользователь не impersonate. Portable/AppX/MSIX и остаточные файлы не детектируются; запись в реестре не доказывает наличие работоспособного приложения.

Нативные вызовы синхронные: context проверяется до/после и между элементами. In-flight Windows API нельзя принудительно прервать. Типизированные структуры x/sys v0.6.0 закреплены (upstream go 1.17); их выбор не подтверждает runtime-совместимость всего проекта с Win7. Обязательны Go 1.20 и VM-проверки.

## Применимость задач

`inventory.Applicable(requirements, system)` сравнивает canonical OS tokens и native architecture. Неизвестные требования/цель дают ошибку; несовпадение — false. Empty requirements разрешают только поддерживаемую workstation matrix.

Перед реальным выполнением оборачивайте адаптер через `inventory.GuardAdapter(system, adapter)` с system из Native.System. Guard проверяет все три фазы. Несовместимый Precheck => NotApplicable, зависимые задачи => Skipped. Guard не проверяет версию установщика/подпись/драйвер: каталог/адаптер обязаны задавать точные требования. Прямой engine.Run без wrapper не читает ОС автоматически. Resume сохраняет доверенные завершённые исходы; live inventory прошлых результатов не перепроверяется.

## CLI

```powershell
go build -o bin/WinDeploy-Inspect-Windows-x64.exe ./cmd/windeploy-inspect
.\bin\WinDeploy-Inspect-Windows-x64.exe inventory --format text
.\bin\WinDeploy-Inspect-Windows-x64.exe inventory --format json --output local/new-inventory.json
.\bin\WinDeploy-Inspect-Windows-x64.exe preview --profiles profiles --profile office
.\bin\WinDeploy-Inspect-Windows-x64.exe preview --profiles profiles --profile office --select sevenzip
```

Каталог output создаёт вызывающий код. Файл создаётся исключительно новым, существующие файлы/симлинки не перезаписываются. Запись в stdout поддерживается. Timeout inventory по умолчанию 15s, кооперативный. Exit codes: 0 полный успех; 1 неверный ввод/output/error; 2 частичный Inventory; 3 отмена/deadline. Даже частичный/отменённый inventory выдаёт отчёт со статусами, если output доступен. Full JSON содержит приватные hostname/IP/MAC/названия ПО; сохраняйте локально, не коммитьте.

Preview загружает до 128 regular YAML-файлов из явно заданного каталога; duplicate IDs, symlinks и ошибки схемы отклоняются. Пустой `--select=` означает ни одного компонента, отсутствие флага — все enabled. Никаких адаптеров, installations или фиктивных Success. Команды execute/install отсутствуют.

CLI — диагностический инструмент для разработки и сквозных проверок, не Modern/Legacy GUI и не готовая portable-поставка WinDeploy. Бинарники не коммитятся.
