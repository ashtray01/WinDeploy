param([string]$Wails = '', [switch]$InstallDependencies, [switch]$Diagnostic)
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
if (!$Wails) { $Wails = Join-Path $taskRoot 'cache/tools/wails-3.1/wails3.exe' }
if (!(Test-Path -LiteralPath $Wails)) { throw 'Install the pinned Wails CLI v3.1.0 first; see docs/modern.md' }
$taskOldCache = $env:GOCACHE
$taskOldModCache = $env:GOMODCACHE
$taskOldOS = $env:GOOS
$taskOldArch = $env:GOARCH
$taskOldWork = $env:GOWORK
function Invoke-ModernStep([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Executable failed: $LASTEXITCODE" }
}
Push-Location (Join-Path $taskRoot 'desktop/modern')
try {
    $env:GOCACHE = Join-Path $taskRoot 'cache/go-build'
    $env:GOMODCACHE = Join-Path $taskRoot 'cache/_go-mod'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    Push-Location frontend
    try {
        if ($InstallDependencies -or !(Test-Path node_modules)) { Invoke-ModernStep npm.cmd @('ci', '--cache', (Join-Path $taskRoot 'cache/npm')) }
        # The committed bindings allow this first build on a clean clone;
        # the embedded dist must exist before Go can load main for bindgen.
        if (!(Test-Path dist/index.html)) { Invoke-ModernStep npm.cmd @('run', 'build') }
    } finally { Pop-Location }
    Invoke-ModernStep $Wails @('generate', 'bindings', '-ts', '-i', '-names', '-noevents', '-d', 'frontend/bindings', '.')
    Push-Location frontend
    try {
        Invoke-ModernStep npm.cmd @('test')
        Invoke-ModernStep npm.cmd @('run', 'build')
    } finally { Pop-Location }
    Invoke-ModernStep go @('test', './...')
    Invoke-ModernStep go @('vet', './...')
    # Include the local replacement as a main module during cache verification.
    # Go 1.26.2 otherwise tries to verify the nonexistent windeploy v0.0.0 zip.
    $taskWorkDir = Join-Path $taskRoot 'local/_modern-verify'
    New-Item -ItemType Directory -Force -Path $taskWorkDir | Out-Null
    $taskWorkFile = Join-Path $taskWorkDir 'go.work'
    $taskWorkRoot = $taskRoot.Replace('\', '/')
    [IO.File]::WriteAllText($taskWorkFile, "go 1.26.0`nuse (`n`"$taskWorkRoot`"`n`"$taskWorkRoot/desktop/modern`"`n)`n")
    $env:GOWORK = $taskWorkFile
    try { Invoke-ModernStep go @('mod', 'verify') } finally { $env:GOWORK = $taskOldWork }
    # Plain go build does not generate Windows resources, so embed the icon and
    # manifest from assets/ before linking the GUI executable.
    Invoke-ModernStep $Wails @('generate', 'syso', '-arch', 'amd64', '-icon', 'assets/windows/icon.ico', '-manifest', 'assets/windows/windeploy.manifest', '-out', 'rsrc_windows_amd64.syso')
    Invoke-ModernStep go @('build', '-tags', 'production', '-ldflags', '-H windowsgui', '-o', (Join-Path $taskRoot 'bin/WinDeploy-Windows-x64.exe'), '.')
    if ($Diagnostic) {
        # A console subsystem preserves panics from dependency init(), before main/log setup.
        Invoke-ModernStep go @('build', '-tags', 'production', '-o', (Join-Path $taskRoot 'bin/WinDeploy-Diagnostic-x64.exe'), '.')
    }
} finally {
    Pop-Location
    $env:GOCACHE = $taskOldCache
    $env:GOMODCACHE = $taskOldModCache
    $env:GOOS = $taskOldOS
    $env:GOARCH = $taskOldArch
    $env:GOWORK = $taskOldWork
}
