param(
    [string]$Go = 'go',
    [string]$BuildCache = 'go-build',
    [string]$TestTemp = '',
    [switch]$AllowNativeWrites
)
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
    # Go 1.20's test binary is confined to the workspace by this host's file
    # sandbox, and ACL tests need permission to change a private test DACL. The
    # default is therefore a workspace directory; CI and normal shells may pass
    # -TestTemp to use any fast local disk. Atomic replacement is used by state
    # and session stores so a OneDrive indexer cannot turn a rename into a failed
    # checkpoint.
    if (!$TestTemp) { $TestTemp = Join-Path $taskRoot 'cache/tmp' }
    $taskTemp = [IO.Path]::GetFullPath($TestTemp)
    New-Item -ItemType Directory -Force -Path $taskTemp | Out-Null
    $env:TEMP = $taskTemp
    $env:TMP = $taskTemp
    $env:GOTMPDIR = $taskTemp
    $env:GOOS = 'windows'
    & $Go version
    if ($LASTEXITCODE -ne 0) { throw 'Go toolchain is unavailable' }
    $taskTestArgs = @('./...', '-count=1')
    if (!$AllowNativeWrites) {
        # Credential Manager and HKCU writes are safe only on a disposable runner.
        $taskTestArgs += @('-skip', 'TestWindowsCredentialStoreRoundTrip|TestRegistryStoreRoundTrip')
    }
    foreach ($taskArch in @('amd64', '386')) {
        $env:GOARCH = $taskArch
        & $Go test @taskTestArgs
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
