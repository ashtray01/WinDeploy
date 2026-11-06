param([string]$Go = 'go', [string]$BuildCache = 'go-build')
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskOldArch = $env:GOARCH
$taskOldOS = $env:GOOS
$taskOldCache = $env:GOCACHE
$taskOldModCache = $env:GOMODCACHE
$taskOldTemp = $env:TEMP
$taskOldTmp = $env:TMP
$taskOldGoTmp = $env:GOTMPDIR
Push-Location $taskRoot
try {
    $env:GOCACHE = Join-Path $taskRoot ('cache\' + $BuildCache)
    $env:GOMODCACHE = Join-Path $taskRoot 'cache\_go-mod'
    # A sandboxed shell can deny the user TEMP directory; keep go test/build
    # temporaries inside the workspace and restore the environment afterwards.
    $taskTemp = Join-Path $taskRoot 'cache\tmp'
    New-Item -ItemType Directory -Force -Path $taskTemp | Out-Null
    $env:TEMP = $taskTemp
    $env:TMP = $taskTemp
    $env:GOTMPDIR = $taskTemp
    $env:GOOS = 'windows'
    & $Go version
    if ($LASTEXITCODE -ne 0) { throw 'Go toolchain is unavailable' }
    foreach ($taskArch in @('amd64', '386')) {
        $env:GOARCH = $taskArch
        & $Go test ./...
        if ($LASTEXITCODE -ne 0) { throw "Core tests failed for $taskArch" }
        & $Go vet ./...
        if ($LASTEXITCODE -ne 0) { throw "Core vet failed for $taskArch" }
        & $Go build ./...
        if ($LASTEXITCODE -ne 0) { throw "Core build failed for $taskArch" }
    }
} finally {
    Pop-Location
    $env:GOARCH = $taskOldArch
    $env:GOOS = $taskOldOS
    $env:GOCACHE = $taskOldCache
    $env:GOMODCACHE = $taskOldModCache
    $env:TEMP = $taskOldTemp
    $env:TMP = $taskOldTmp
    $env:GOTMPDIR = $taskOldGoTmp
}
