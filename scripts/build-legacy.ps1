param(
    [string]$Go = "",
    [switch]$GuiSmoke
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
if (-not $Go) { $Go = Join-Path $root "local/_toolchains/go1.20.14/go/bin/go.exe" }
if (-not (Test-Path -LiteralPath $Go)) { throw "Go 1.20.14 not found: $Go" }
$bin = Join-Path $root "bin"
$legacy = Join-Path $root "legacy"
$temp = Join-Path $root "local"
New-Item -ItemType Directory -Force -Path $bin, $temp | Out-Null

$env:GOOS = "windows"
$env:GOTOOLCHAIN = "local"
$env:TEMP = Join-Path $root "cache/tmp"
$env:TMP = $env:TEMP
$env:GOTMPDIR = $env:TEMP
$env:GOCACHE = Join-Path $root "cache/go-build-go120"
$env:GOMODCACHE = Join-Path $root "cache/_go-mod"
New-Item -ItemType Directory -Force -Path $env:TEMP | Out-Null

Push-Location $legacy
try {
    foreach ($arch in @("amd64", "386")) {
        Write-Host "Legacy ${arch}: test"
        $env:GOARCH = $arch
        & $Go test ./... 2>&1 | ForEach-Object { $_ }
        if ($LASTEXITCODE -ne 0) { throw "legacy tests failed for $arch" }

        $suffix = if ($arch -eq "amd64") { "x64" } else { "x86" }
        $exe = Join-Path $bin "WinDeploy-Legacy-Windows-$suffix.exe"
        Write-Host "Legacy ${arch}: build $exe"
        & $Go build -trimpath -ldflags "-H windowsgui -s -w" -o $exe .
        if ($LASTEXITCODE -ne 0) { throw "legacy build failed for $arch" }

        $console = Join-Path $temp "legacy-selftest-$arch.exe"
        & $Go build -trimpath -o $console .
        if ($LASTEXITCODE -ne 0) { throw "legacy console build failed for $arch" }
        & $console --selftest
        if ($LASTEXITCODE -ne 0) { throw "legacy selftest failed for $arch" }
        if ($GuiSmoke) {
            & $console --smoke
            if ($LASTEXITCODE -ne 0) { throw "legacy GUI smoke failed for $arch" }
        }
        Remove-Item -LiteralPath $console -Force -ErrorAction SilentlyContinue
    }
} finally {
    Pop-Location
}

foreach ($name in @("WinDeploy-Legacy-Windows-x64.exe", "WinDeploy-Legacy-Windows-x86.exe")) {
    $path = Join-Path $bin $name
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $path).Hash.ToLowerInvariant()
    Write-Host ("{0}  {1} bytes  SHA256 {2}" -f $name, (Get-Item -LiteralPath $path).Length, $hash)
}
