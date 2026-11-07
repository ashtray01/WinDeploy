# Сборка и выпуск

## Локальная сборка

```powershell
# Core + Legacy + portable/offline ZIP
scripts/package-release.ps1 -Version 0.1.0-preview.1

# С Modern (нужен cache/tools/wails-3.1/wails3.exe)
scripts/package-release.ps1 -Version 0.1.0-preview.1 -BuildModern
```

Результат — `dist/`:

- `WinDeploy-<version>-portable-x64.zip` — Modern x64, Legacy x64, CLI inspect/packages/run, docs/README.
- `WinDeploy-<version>-portable-x86.zip` — Legacy x86 и x86 CLI.
- `WinDeploy-<version>-offline-x64.zip` — portable + `catalog/packages.yaml` + `Packages/` с проверенными установщиками (7-Zip, SumatraPDF, PrinterOne на момент сборки) + offline README.
- `SHA256SUMS.txt`, `RELEASE-NOTES.md`.

Offline-файлы ищутся в `cache/Downloads/artifacts` и `local/offline-cache/sha256` по SHA-256 из `catalog/packages.yaml`; скрипт кладёт их под именами, которые ожидает каталог. Остальные программы нужно скачать `windeploy-packages download` на машине с сетью или добавить вручную с совпадающим хешем.

## CI

`.github/workflows/ci.yml`:

- Core: Go 1.20.14, amd64+386 test/vet/build через `scripts/test-core.ps1`.
- Legacy: Go 1.20.14, тесты, selftest, сборка x64/x86 с манифестом.
- Modern: Go 1.26.2, npm ci/typecheck/vitest/build, Go test/vet.
- Modern Wails EXE: установка pinned `wails3@v3.1.0`, полная сборка `scripts/build-modern.ps1`.

## Release

`.github/workflows/release.yml` запускается по тегу `v*` или вручную. Собирает core/legacy, Modern Wails, пересобирает ZIP, публикует **pre-release** с SHA256SUMS. Требуется GitHub remote и права `contents: write`; локально публикация не выполнялась.

## Критерии v0.1.0

Сейчас статус **preview, не публиковался**:

- [x] Core Go 1.20 amd64/386 test/vet/build.
- [x] Modern 37 UI-тестов, strict TypeScript, Vite, service tests, Wails EXE собирается.
- [x] Legacy x64/x86 test/vet/selftest + реальное окно на Windows 11.
- [x] Portable/offline ZIP и SHA256SUMS.
- [ ] Modern GUI smoke на машине с работающим WebView2.
- [ ] Windows 10/11 (включая выбранные LTSC) VM; Windows 7 VM — FROZEN и не gate Modern.
- [ ] Реальные установки ПО/драйверов/принтеров, SMB, политики и настройки.
- [ ] GitHub remote/release.

До выполнения этих пунктов выпускать стабильный v0.1.0 нельзя; текущие ZIP — preview-артефакты.

## Стратегия после аудита 2026-11-07
Modern — основной продукт. Legacy код и описанные выше исторические bundles сохранены, но дальнейшая Legacy-разработка заморожена. Устранение P0 из AUDIT.md и реальный Modern MVP E2E обязательны до стабильного выпуска. Текущие workflows ещё связаны с Legacy packaging и не обеспечивают эти gates автоматически; разделение Modern release запланировано в TASKS.md.
