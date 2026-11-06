# Проверка общего ядра Go 1.20

Root go.mod сохраняет `go 1.20`, Wails в его зависимостях отсутствует. Для независимой проверки доступна команда:

```powershell
.\scripts\test-core.ps1 -Go .\local\_toolchains\go1.20.14\go\bin\go.exe -BuildCache go-build-go120
```

Скрипт печатает фактическую версию компилятора, запускает test/vet/build корневого модуля на windows/amd64 и windows/386 и восстанавливает переменные окружения. Modern является вложенным модулем и намеренно не входит в эту проверку.

Официальный Go 1.20.14 находится только в local/_toolchains; системный Go/PATH не меняются. Download завершён через dl.google.com/curl resume после обрыва первого соединения. Полный SHA-256 проверен по официальной metadata до распаковки. Подчёркивание в _toolchains исключает собственные test/исходники Go из root ./...; старый toolchain использует отдельный build cache. Фактический финальный результат фиксируется в HANDOFF/отчёте.

Даже успешные Go 1.20 test/build на текущей Windows не подтверждают запуск Win7 SP1 — нужна соответствующая VM.

Фактическая проверка завершена: **Go 1.20.14 windows/amd64**, root `go test ./...`, `go vet ./...`, `go build ./...` — PASS для GOARCH=amd64 и GOARCH=386. Native Inventory tests реально исполнились на текущей Windows. Root go.mod сохранил Go 1.20 и исходные зависимости; Wails в root module graph не добавлен.
