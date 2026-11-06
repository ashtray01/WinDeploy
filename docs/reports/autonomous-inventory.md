# WinDeploy — итог автономной сессии Inventory

Выполнены 10 функциональных задач, каждая отдельным коммитом. Исходное состояние: 1bc74b6, базовый engine/state/profiles без Inventory. Новое правило AGENTS требует общей проверки после каждых 10–15 завершённых задач; эта проверка выполнена после десятой задачи.

| Задача | Реализация | Коммит |
| --- | --- | --- |
| Windows identity | Hostname, NT version/build/SP/product type, native/process architecture, исключение Server/Win7 RTM из supported tokens | 6e5502a |
| CPU | Read-only model/vendor, active logical processors across groups | accdab0 |
| RAM | GlobalMemoryStatusEx, 64-bit total/available physical bytes | a2c1351 |
| Volumes | Local fixed/ramdisk capacity, quota-aware available bytes, явная неизвестная capacity остальных типов | 5c7c916 |
| Interfaces | IPv4/IPv6 prefixes, flags, MTU, MAC, частичные ошибки | c8f92d0 |
| Adapter configuration | Gateways, DNS, DHCPv4, link speeds, IPv6 scopes | 339bf25 |
| Installed apps | Видимые HKLM/HKCU entries, 32/64 registry views, точный dedup без Win32_Product | 7e2f45b |
| Collector | Семь секций, Complete/ErrIncomplete, partial/cancelled/not-collected, без raw errors | 47af203 |
| Applicability | Проверка canonical OS/architecture, guard всех фаз engine.Adapter | 3ce3b5f |
| Read-only CLI | inventory text/JSON, safe file output, profiles preview и explicit exit codes | 1ff69d4 |

HANDOFF checkpoints созданы после задач 3, 6 и 9: a5a4c49, 64efec9, 398807c. Итоговая документация сохраняется отдельным docs-коммитом (HEAD сессии).

## Фактически выполненные проверки

| Проверка | Результат |
| --- | --- |
| Unit/native tests соответствующего пакета после каждой задачи | PASS |
| `go test ./...` windows/amd64 | PASS |
| `go vet ./...` windows/amd64 | PASS |
| `go build ./...` windows/amd64 | PASS |
| `go test ./...` windows/386 | PASS, тесты реально запущены на текущей x64 Windows |
| `go build ./...` windows/386 | PASS |
| `GOOS=linux GOARCH=amd64 go build ./...` | PASS, только cross-build |
| `go mod verify` | all modules verified |
| gofmt / git diff --check | PASS |
| Реальная x64 CLI inventory | Complete=true, 7 секций, 3 volumes, 7 interfaces, 167 visible app entries |
| Независимая проверка identity | Windows build совпал с CurrentBuildNumber в реестре, hostname — с Environment.MachineName |
| Реальная x86 CLI inventory | Complete=true, native=amd64, process=386, 7 секций, те же 167 app entries |
| Реальные CLI previews | Все 8 примеров профилей, scope structure-and-dag-only, без execution |

Сборки: `bin/WinDeploy-Inspect-Windows-x64.exe`, `bin/WinDeploy-Inspect-Windows-x86.exe` (игнорируются Git). Полный smoke JSON содержит приватные метаданные и оставлен только в игнорируемом local/. В отчёт не включены hostname/IP/MAC/названия установленного ПО.

## Что тесты доказывают и чего не доказывают

Native integration-тесты реально читают все семь секций на доступной Windows. Unit-тесты отдельно проверяют классификацию ОС, native layouts, значения >4 GiB, parsing, partial errors, cancellation, dedup registry views, DNS/gateway/IPv6 scope decoding, блокировку несовместимого Execute и CLI output failures. Проверки не выполняют установки, сетевую перенастройку, elevation или uninstall.

Использован Go 1.26.2. Go 1.20 toolchain и Windows 7 SP1 VM не проверены. Эти EXE собраны текущим toolchain и не являются Win7 Legacy-сборками. Нет доказательства запуска на всей целевой матрице Windows 10/11/7. Race detector не запускался: gcc/clang не найдены в PATH.

Inventory ограничен описанными источниками: нет физических дисков/SMART, AppX/MSIX/portable detection, DHCPv6 leases, маршрутов или тестов reachability. HKCU относится к эффективному пользователю процесса. Синхронные Windows API отменяются кооперативно. Registry entries не доказывают фактическую исправность приложения.

GuardAdapter должен использовать вызывающий код перед реальными изменяющими систему адаптерами; engine.Run напрямую не читает ОС. Совместимость конкретного дистрибутива/драйвера всё ещё обязан проверять будущий каталог и установочный адаптер.

## Продолжение

Этап 3 выполнен по критерию фактических данных текущей среды и тестов обработки ошибок; целевая VM-матрица остаётся открытой. Следующая задача — проверить актуальный статус/требования Wails и создать отдельный Modern-модуль, сохраняя core go.mod на Go 1.20. Затем подключить реальный Inventory и profiles preview к русскому dark dashboard без фиктивных Success.

Сессия не содержит GUI или готового установочного продукта. Увеличение пакета до 15–20 задач следует оценивать по сложности следующих GUI-задач; по этому отчёту нельзя достоверно оценить остаток токенов или количественную экономию лимитов.
