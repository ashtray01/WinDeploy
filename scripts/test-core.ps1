param([string]$Go = 'go', [string]$BuildCache = 'go-build')
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskOldArch = $env:GOARCH
$taskOldOS = $env:GOOS
$taskOldCache = $env:GOCACHE
$taskOldModCache = $env:GOMODCACHE
Push-Location $taskRoot
try {
    $env:GOCACHE = Join-Path $taskRoot ('cache\' + $BuildCache)
    $env:GOMODCACHE = Join-Path $taskRoot 'cache\_go-mod'
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
}
