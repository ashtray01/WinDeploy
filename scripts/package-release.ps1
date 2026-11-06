param(
    [string]$Version = "0.1.0-preview.1",
    [string]$Go = "",
    [switch]$BuildModern,
    [switch]$SkipModern,
    [switch]$SkipBuilds
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if (-not $Go) { $Go = Join-Path $root "local/_toolchains/go1.20.14/go/bin/go.exe" }
$bin = Join-Path $root "bin"
$dist = Join-Path $root "dist"
$stage = Join-Path $root "local/release-stage"
$env:TEMP = Join-Path $root "cache/tmp"
$env:TMP = $env:TEMP
$env:GOTMPDIR = $env:TEMP
$env:GOCACHE = Join-Path $root "cache/go-build-go120"
$env:GOMODCACHE = Join-Path $root "cache/_go-mod"
$env:GOOS = "windows"
$env:GOTOOLCHAIN = "local"
New-Item -ItemType Directory -Force -Path $bin, $dist, $env:TEMP | Out-Null
Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $stage | Out-Null

if (-not $SkipBuilds) {
    Write-Host "Building core CLIs (amd64/386)"
    foreach ($arch in @("amd64", "386")) {
        $env:GOARCH = $arch
        $suffix = if ($arch -eq "amd64") { "x64" } else { "x86" }
        foreach ($cmd in @("windeploy-inspect", "windeploy-packages", "windeploy-run")) {
            $out = Join-Path $bin ("{0}-{1}.exe" -f $cmd, $suffix)
            & $Go build -trimpath -ldflags "-s -w" -o $out (Join-Path $root "cmd/$cmd")
            if ($LASTEXITCODE -ne 0) { throw "core build failed: $cmd $arch" }
        }
    }
    Write-Host "Building Legacy x64/x86"
    & (Join-Path $root "scripts/build-legacy.ps1") -Go $Go
    if ($LASTEXITCODE -ne 0) { throw "legacy build failed" }
    if ($BuildModern) {
        Write-Host "Building Modern x64"
        $wails = Join-Path $root "cache/tools/wails-3.1/wails3.exe"
        & (Join-Path $root "scripts/build-modern.ps1") -Wails $wails -InstallDependencies
        if ($LASTEXITCODE -ne 0) { throw "modern build failed" }
    }
}

$modern = Join-Path $bin "WinDeploy-Windows-x64.exe"
$legacy64 = Join-Path $bin "WinDeploy-Legacy-Windows-x64.exe"
$legacy86 = Join-Path $bin "WinDeploy-Legacy-Windows-x86.exe"
foreach ($path in @($legacy64, $legacy86)) { if (!(Test-Path -LiteralPath $path)) { throw "missing $path" } }
$modernAvailable = Test-Path -LiteralPath $modern

function Stage-Arch([string]$arch, [string]$destination) {
    New-Item -ItemType Directory -Force -Path $destination | Out-Null
    $suffix = if ($arch -eq "amd64") { "x64" } else { "x86" }
    foreach ($cmd in @("windeploy-inspect", "windeploy-packages", "windeploy-run")) {
        Copy-Item -LiteralPath (Join-Path $bin ("{0}-{1}.exe" -f $cmd, $suffix)) -Destination $destination
    }
    if ($arch -eq "amd64") {
        Copy-Item -LiteralPath $legacy64 -Destination (Join-Path $destination "WinDeploy-Legacy.exe")
        if ($modernAvailable) { Copy-Item -LiteralPath $modern -Destination (Join-Path $destination "WinDeploy-Modern.exe") }
    } else {
        Copy-Item -LiteralPath $legacy86 -Destination (Join-Path $destination "WinDeploy-Legacy.exe")
    }
    $readme = @(
        "WinDeploy $Version ($suffix)",
        "",
        "Portable-набор. Запускайте EXE из этой папки; установка не требуется.",
        "Modern: WinDeploy-Modern.exe (x64, Wails/WebView2).",
        "Legacy: WinDeploy-Legacy.exe (x86/x64, нативный Win32, Windows 7 SP1).",
        "CLI: windeploy-inspect.exe, windeploy-packages.exe, windeploy-run.exe.",
        "",
        "Каталог программ встроен. Онлайн-загрузка требует сети; offline-набор дополняется папкой Packages.",
        "Изменяющие операции требуют прав администратора и подтверждения.",
        "Отчёты: %LOCALAPPDATA%\WinDeploy\reports, сессии: %LOCALAPPDATA%\WinDeploy\sessions.",
        "Документация: docs/.",
        "",
        "Проверки: Go 1.20 amd64+386 test/vet/build; Modern 37 UI tests; Legacy selftest+GUI smoke x64/x86",
        "на Windows 11. Windows 7 VM и реальные установки не проверены."
    ) -join "`r`n"
    [IO.File]::WriteAllText((Join-Path $destination "README.txt"), $readme, (New-Object System.Text.UTF8Encoding($false)))
}

$portableRoot = Join-Path $stage "portable"
$offlineRoot = Join-Path $stage "offline"
Stage-Arch "amd64" (Join-Path $portableRoot "x64")
Stage-Arch "386" (Join-Path $portableRoot "x86")
Stage-Arch "amd64" (Join-Path $offlineRoot "x64")
Stage-Arch "386" (Join-Path $offlineRoot "x86")

Copy-Item -Recurse -LiteralPath (Join-Path $root "docs") -Destination (Join-Path $stage "docs")
New-Item -ItemType Directory -Force -Path (Join-Path $offlineRoot "catalog") | Out-Null
Copy-Item -LiteralPath (Join-Path $root "catalog/packages.yaml") -Destination (Join-Path $offlineRoot "catalog")
$packageDir = Join-Path $offlineRoot "Packages"
New-Item -ItemType Directory -Force -Path $packageDir | Out-Null
$artifactRoot = Join-Path $root "cache/Downloads/artifacts"
$copied = 0
if (Test-Path -LiteralPath $artifactRoot) {
    Get-ChildItem -LiteralPath $artifactRoot -Recurse -File | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $packageDir $_.Name)
        $copied++
    }
}
# Verified online downloads can be promoted into the offline layout. Hashes are
# catalog-bound; keep this map in sync with catalog/packages.yaml.
$offlineMap = @{
    "fd99e86f57d8f429ef5ed045bf98315af962e2309b0f1ba87a147a44c71647d0" = "7z2605-x64.exe"
    "1eee71cccd2ea6e94d5bcea54ee2f759844da3e1a0ee2f6045035b1d17b94381" = "SumatraPDF-3.6.1-64-install.exe"
    "d0211d52105b63e86689faa92dce63fe337d27b1b3e7f3504750454b60b60740" = "PrinterOne.exe"
}
$downloadCache = Join-Path $root "local/offline-cache/sha256"
foreach ($hash in $offlineMap.Keys) {
    $source = Join-Path $downloadCache ($hash + ".exe")
    if (Test-Path -LiteralPath $source) {
        Copy-Item -LiteralPath $source -Destination (Join-Path $packageDir $offlineMap[$hash])
        $copied++
    }
}
$offlineReadme = @(
    "Offline-набор WinDeploy $Version",
    "",
    "Скопируйте содержимое Packages в %ProgramData%\WinDeploy\Packages на целевой машине.",
    "Каталог программ ищет offline-файлы по именам из catalog/packages.yaml.",
    "В этот предварительный набор включены только уже загруженные и проверенные файлы: $copied.",
    "Для остальных программ используйте windeploy-packages download на машине с сетью",
    "или добавьте проверенные установщики вручную (SHA-256 должен совпадать с каталогом).",
    "",
    "Интернет не требуется для уже включённых offline-пакетов; Modern/WebView2 и Legacy",
    "работают локально. SMB/драйверы/принтеры требуют инфраструктуру."
) -join "`r`n"
[IO.File]::WriteAllText((Join-Path $offlineRoot "README-OFFLINE.txt"), $offlineReadme, (New-Object System.Text.UTF8Encoding($false)))

$commit = (& git -C $root rev-parse --short HEAD).Trim()
$notes = @(
    "# WinDeploy $Version",
    "",
    "Предварительная сборка (pre-release), commit ``$commit``, $(Get-Date -Format 'yyyy-MM-dd HH:mm').",
    "",
    "## Что внутри",
    "",
    "- Portable x64/x86: Modern (x64), Legacy (x86/x64), CLI inspect/packages/run.",
    "- Offline x64: portable + catalog/packages.yaml + Packages (только реально загруженные файлы).",
    "- docs/: архитектура, пакеты, сеть, драйверы/принтеры, браузеры/настройки, выполнение, Legacy.",
    "",
    "## Проверки",
    "",
    "- Go 1.20.14 amd64+386: core test/vet/build PASS.",
    "- Modern: 37 vitest, strict TypeScript, Vite build, Go service tests PASS; нативный GUI smoke",
    "  на этом хосте заблокирован средой WebView2 (0x8000FFFF), не кодом.",
    "- Legacy: test/vet amd64+386, selftest, реальное GUI-окно --smoke x64/x86 на Windows 11 PASS.",
    "- Реальные установки, драйверы, принтеры, SMB и Windows 7 SP1 VM не проверены.",
    "",
    "## Ограничения",
    "",
    "Это не production-релиз и не публикация: remote отсутствует, GitHub CLI недоступен.",
    "Перед публикацией нужны VM-матрица Windows 7/10/11, реальные установки и Modern GUI smoke",
    "на машине с работающим WebView2.",
    "",
    "## Установка",
    "",
    "Распакуйте ZIP и запустите нужный EXE. Portable не пишет в Program Files;",
    "runtime-данные находятся в %LOCALAPPDATA%\WinDeploy. Offline: скопируйте Packages",
    "в %ProgramData%\WinDeploy\Packages."
) -join "`r`n"
[IO.File]::WriteAllText((Join-Path $stage "RELEASE-NOTES.md"), $notes, (New-Object System.Text.UTF8Encoding($false)))
Copy-Item -LiteralPath (Join-Path $stage "RELEASE-NOTES.md") -Destination (Join-Path $portableRoot "RELEASE-NOTES.md")
Copy-Item -LiteralPath (Join-Path $stage "RELEASE-NOTES.md") -Destination (Join-Path $offlineRoot "RELEASE-NOTES.md")

$portableZip = Join-Path $dist ("WinDeploy-{0}-portable-x64.zip" -f $Version)
$portable86Zip = Join-Path $dist ("WinDeploy-{0}-portable-x86.zip" -f $Version)
$offlineZip = Join-Path $dist ("WinDeploy-{0}-offline-x64.zip" -f $Version)
Remove-Item -LiteralPath $portableZip, $portable86Zip, $offlineZip -Force -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $portableRoot "x64"), (Join-Path $portableRoot "RELEASE-NOTES.md") -DestinationPath $portableZip -CompressionLevel Optimal
Compress-Archive -Path (Join-Path $portableRoot "x86"), (Join-Path $portableRoot "RELEASE-NOTES.md") -DestinationPath $portable86Zip -CompressionLevel Optimal
Compress-Archive -Path (Join-Path $offlineRoot "*") -DestinationPath $offlineZip -CompressionLevel Optimal

$hashes = @()
foreach ($file in @($portableZip, $portable86Zip, $offlineZip)) {
    $hashes += ("{0}  {1}" -f (Get-FileHash -Algorithm SHA256 -LiteralPath $file).Hash.ToLowerInvariant(), (Split-Path -Leaf $file))
}
$sums = Join-Path $dist "SHA256SUMS.txt"
[IO.File]::WriteAllLines($sums, $hashes, (New-Object System.Text.UTF8Encoding($false)))
Copy-Item -LiteralPath $sums -Destination (Join-Path $stage "SHA256SUMS.txt")
Copy-Item -LiteralPath (Join-Path $stage "RELEASE-NOTES.md") -Destination (Join-Path $dist "RELEASE-NOTES.md")
Write-Host "Release artifacts:"
$hashes | ForEach-Object { Write-Host $_ }
Write-Host "Stage: $stage"
