# Сборка и выпуск Modern beta

## Единый номер версии

Канонический номер — `VERSION` (сейчас `0.1.0-beta.1`). Он проверяется в
`desktop/modern/frontend/package.json`, `package-lock.json`, `CHANGELOG.md` и
release notes. При сборке номер внедряется в Go (`internal/buildinfo`), UI и
Windows version resource EXE. Не создавайте второй hard-coded номер в коде/UI.

```powershell
./scripts/verify-version.ps1
```

## Локальная Modern-сборка

Нужны закреплённые Wails v3.1.0, Go 1.26.2 и Node.js 24.x.

```powershell
./scripts/build-modern.ps1 -Diagnostic
```

Скрипт:

1. генерирует bindings;
2. запускает 46 UI tests, strict TypeScript и production Vite build;
3. запускает Modern Go tests/vet и module verification;
4. генерирует Windows icon/manifest/version resource;
5. собирает `bin/WinDeploy-Windows-x64.exe` с `-trimpath -buildvcs=false` и
   внедрённой версией/commit.

`-Diagnostic` дополнительно создаёт console EXE для ранней диагностики. Обычная
GUI-сборка использует production assets и не требует Go/Node.js на целевой
машине.

## Артефакты beta

```powershell
# Собрать и упаковать
./scripts/package-modern-release.ps1

# Либо упаковать уже собранный EXE
./scripts/package-modern-release.ps1 -SkipBuild
```

Результат:

- `dist/WinDeploy-Windows-x64.exe`;
- `dist/WinDeploy-0.1.0-beta.1-portable-x64.zip` (EXE, README, docs, known issues);
- `dist/SHA256SUMS.txt` — фактические SHA-256;
- `dist/RELEASE-NOTES.md`.

Скрипт проверяет версию EXE, создаёт portable ZIP и повторно сверяет SHA-256.
Code signing не настроен: EXE имеет статус `NotSigned`; это должно быть указано в
release notes и Known Issues. Не заявлять подпись без сертификата.

## CI

`.github/workflows/ci.yml` — Modern-only:

- Core Go 1.20.14: test/vet/build amd64+386.
- Modern: Go 1.26.2, Node 24, typecheck/tests/build, service tests/vet.
- Modern EXE + release dry-run: build, verify version, SHA-256, upload artifacts.

`.github/workflows/legacy-ci.yml` — Legacy FROZEN, только `workflow_dispatch`.
Legacy не входит в Modern CI и не блокирует beta.

## Release workflow

`.github/workflows/release.yml` запускается по тегу `v*` или вручную:

1. проверяет, что тег равен `v$(VERSION)`;
2. выполняет Core/Modern проверки;
3. собирает EXE и пакет;
4. загружает artifacts;
5. по тегу создаёт **draft pre-release** (`draft: true`, `prerelease: true`).

Workflow не публикует Release автоматически. Владелец проверяет draft, SHA-256,
release notes и вручную публикует его. До явного подтверждения владельца не
открывать репозиторий и не создавать публичный Release.

## Legacy

Windows 7 Legacy FROZEN. Старый `scripts/package-release.ps1` сохранён для
исторических bundle-ов, но Modern beta не зависит от Legacy. Не запускайте
Legacy-сборку как release gate Modern.

## Перед публикацией

См. [PUBLICATION_CHECKLIST.md](PUBLICATION_CHECKLIST.md). Лицензия, публичный
owner/repository URL, проверка Git-истории и подтверждение владельца — обязательные
условия. Текущий статус готовности — [BETA_READINESS.md](BETA_READINESS.md).
