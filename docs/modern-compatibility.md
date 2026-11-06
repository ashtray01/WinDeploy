# Modern GUI — изоляция и совместимость

Общий модуль остаётся `go 1.20`, Wails в него не добавляется. Modern — отдельный `desktop/modern/go.mod`, локальный replace на root. Проверки root `./...` не включают вложенный модуль: Modern проверяется отдельно.

На момент сессии Go proxy публикует **Wails v3.1.0** как последний стабильный v3 (2026-11-01, tag v3.1.0). Зависимость и CLI фиксируются на этой версии, npm runtime — 3.1.0. Installation docs ещё приводят v3.0.0 в примере, поэтому версия перепроверена через `https://proxy.golang.org/github.com/wailsapp/wails/v3/@latest`.

Upstream v3.1.0 go.mod требует Go 1.26.0 (предпочитаемый toolchain go1.26.10). В доступной среде Go 1.26.2; требование языка удовлетворено. Только Modern использует этот toolchain и новые транзитивные зависимости. Root go.mod/go.sum не обновляются ради UI.

Windows desktop требует WebView2; в текущей среде найден Evergreen 156.0.4314.58. Runtime не устанавливается автоматически кодом WinDeploy. Цель Modern — Windows 10/11 x64; текущий запуск не подтверждает всю матрицу версий Windows и зависимостей WebView2. Windows 7 относится к отдельной Legacy-редакции без Wails.

Официальные источники: [Wails installation](https://v3.wails.io/getting-started/installation/), [Wails v3.1.0 go.mod](https://github.com/wailsapp/wails/blob/v3.1.0/v3/go.mod). Фактические результаты Go 1.20 и GUI smoke фиксируются в HANDOFF и итоговом отчёте, не подменяются сборкой Go 1.26.
