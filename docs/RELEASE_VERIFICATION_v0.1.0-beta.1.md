# Проверка публичного выпуска v0.1.0-beta.1

Дата: 2026-11-08 (Europe/Samara). GitHub published_at: 2026-11-07T22:46:17Z.

- [Публичный Pre-release](https://github.com/ashtray01/WinDeploy/releases/tag/v0.1.0-beta.1)
- [Modern CI — PASS, все 3 jobs](https://github.com/ashtray01/WinDeploy/actions/runs/42421280314)
- [Release workflow — PASS](https://github.com/ashtray01/WinDeploy/actions/runs/42421767518)
- Release commit: `56fe16351720b4a74a5e773a7a26e270fe4bbf84`.
- Annotated unsigned tag object: `9a90f754d75ac616d0a27f37e4903ed4839bcd60`.
- GitHub Release ID: `437891980`; `draft=false`, `prerelease=true`.

## Проверки

Core Go 1.20.14 tests/vet/build amd64+386, Modern Go 1.26.2 tests/vet,
frontend tests/strict typecheck/production build, Wails v3.1.0 production EXE,
portable packaging и workflow artifact upload — PASS. Legacy не запускался.
Release workflow дополнительно проверил Git history secret preflight и
соответствие VERSION/tag; draft создан штатным workflow.

Все четыре asset сначала скачаны из draft через authenticated GitHub API.
SHA-256 совпали с SHA256SUMS и GitHub asset digest. После публикации повторно
скачаны без авторизации по browser_download_url; хеши не изменились.
Публичная HTML-страница вернула HTTP 200 и содержит Pre-release; публичный API
подтвердил draft=false/prerelease=true. Тег после публикации не перемещался.

EXE ProductVersion/FileVersion = 0.1.0-beta.1, содержит ожидаемый embedded commit,
Authenticode = NotSigned, production debug flags smoke-port/remote-debugging-port
отсутствуют. ZIP содержит тот же EXE, MIT LICENSE, совпадающие Release Notes и
BUILD-INFO с правильными version/commit/windows/amd64. Diagnostic/Legacy EXE и
runtime logs не включены. Release body совпадает с RELEASE-NOTES.md.

## SHA-256 скачанных публичных assets

```text
b9e210e5e4cade5b24346ebe4f7a17f53e3a6bda5823fa53cd2ca097addee822  WinDeploy-Windows-x64.exe
f2122d90bc4ee63753758649cc2af06e18116ed9734b92979710159bb98653a6  WinDeploy-0.1.0-beta.1-portable-x64.zip
7c72d422cc3ba4e49f9bce14b1687916e3ef92effa73103e7cc833add1332776  RELEASE-NOTES.md
17bbdcb68727733376fc4f78c98b595c9319e98f713eba82629a96626ea2b2aa  SHA256SUMS.txt
```

SHA256SUMS содержит первые три файла; хеш самого manifest записан здесь отдельно.
Автоматические source archives GitHub не относятся к этим четырём release assets.

## Повторная проверка при завершении документации — 2026-11-08

- Публичный API: Release ID `437891980`, `draft=false`, `prerelease=true`,
  `tag_name=v0.1.0-beta.1`, `target_commitish=main`; HTML страница HTTP 200.
- Annotated tag `9a90f754d75ac616d0a27f37e4903ed4839bcd60` указывает на
  `56fe16351720b4a74a5e773a7a26e270fe4bbf84`; tag не перемещался.
- Все четыре asset повторно скачаны без авторизации. Фактические SHA-256 совпали
  с GitHub digest, SHA256SUMS и таблицей выше.
- ZIP содержит ровно один EXE (`WinDeploy-Windows-x64.exe`), его SHA-256
  совпадает с отдельным asset; BUILD-INFO содержит `0.1.0-beta.1`,
  `56fe16351720b4a74a5e773a7a26e270fe4bbf84`, windows/amd64, `NotSigned`.
- Release body совпадает с `RELEASE-NOTES.md` (5323 bytes после нормализации
  переводов строк).
- CI run `42421280314` и release run `42421767518` — `completed/success` на
  `56fe163...`; Issues/Releases доступны без авторизации, открытых Issues — 0.
- Runtime/UI в этой проверке не менялись, скачанный EXE не запускался.


## Границы доказательств

Windows 11 x64 — проверенная платформа по предыдущему actual GUI/Inventory
smoke. Runtime/UI в этой итерации не менялись, скачанный EXE не запускался;
CI build не является новым desktop/bridge smoke. Windows 10/LTSC не проверены,
Windows 7 не поддерживается Modern release. Package-install/SMB acceptance и
real reboot не выполнены; drivers/printers/policies/Windows settings остаются
экспериментальными. Отмена не гарантирует остановку дочерних installer processes.

Владелец явно разрешил этот публичный Pre-release с раскрытыми ограничениями.
B1/B2 не закрыты фиктивным PASS. Подпись EXE и Git-тега не настроена. MIT выбран.
Полный security/license dependency audit и reproducible double-build comparison
не выполнялись. После публикации обновлены документы в main; исходный tag и
опубликованные assets не переписывались.
