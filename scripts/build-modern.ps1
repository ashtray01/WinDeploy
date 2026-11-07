param(
    [string]$Wails = '',
    [switch]$InstallDependencies,
    [switch]$Diagnostic,
    [string]$Version = '',
    [string]$Commit = ''
)
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
if (!$Wails) { $Wails = Join-Path $taskRoot 'cache/tools/wails-3.1/wails3.exe' }
if (!(Test-Path -LiteralPath $Wails)) { throw 'Install the pinned Wails CLI v3.1.0 first; see docs/modern.md' }

if (!$Version) {
    $versionFile = Join-Path $taskRoot 'VERSION'
    if (!(Test-Path -LiteralPath $versionFile)) { throw 'VERSION file is missing' }
    $Version = (Get-Content -LiteralPath $versionFile -Raw).Trim()
}
if ($Version -notmatch '^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$') {
    throw "Invalid semantic version: $Version"
}
if (!$Commit) {
    $Commit = (& git -C $taskRoot rev-parse --verify HEAD 2>$null)
    if ($LASTEXITCODE -ne 0 -or !$Commit) { $Commit = 'dev' }
    $Commit = $Commit.Trim()
}
$packageFile = Join-Path $taskRoot 'desktop/modern/frontend/package.json'
$package = Get-Content -LiteralPath $packageFile -Raw | ConvertFrom-Json
if ($package.version -ne $Version) {
    throw "frontend package.json version $($package.version) does not match VERSION $Version"
}

$taskOldCache = $env:GOCACHE
$taskOldModCache = $env:GOMODCACHE
$taskOldOS = $env:GOOS
$taskOldArch = $env:GOARCH
$taskOldWork = $env:GOWORK
$taskOldTemp = $env:TEMP
$taskOldTmp = $env:TMP
$taskOldGoTmp = $env:GOTMPDIR
function Invoke-ModernStep([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Executable failed: $LASTEXITCODE" }
}
function Write-InfoJSON([string]$Path, [string]$ProductVersion) {
    $numeric = ($ProductVersion -split '-')[0] + '.0'
    $data = [ordered]@{
        fixed = [ordered]@{ file_version = $numeric; product_version = $numeric; flags = 'Prerelease' }
        info = [ordered]@{
            '0409' = [ordered]@{
                FileVersion = $ProductVersion
                ProductVersion = $ProductVersion
                CompanyName = 'WinDeploy'
                FileDescription = 'WinDeploy workstation preparation'
                LegalCopyright = 'WinDeploy contributors'
                ProductName = 'WinDeploy'
                Comments = 'Public Beta - software under active development'
            }
        }
    }
    $dir = Split-Path -Parent $Path
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    [IO.File]::WriteAllText($Path, ($data | ConvertTo-Json -Depth 5), (New-Object System.Text.UTF8Encoding($false)))
}
Push-Location (Join-Path $taskRoot 'desktop/modern')
try {
    $env:GOCACHE = Join-Path $taskRoot 'cache/go-build'
    $env:GOMODCACHE = Join-Path $taskRoot 'cache/_go-mod'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'
    # Trust tests alter private ACLs; OneDrive rejects that. Keep test temp on
    # ordinary local storage so the same command is reproducible outside the repo.
    $taskLocal = [Environment]::GetFolderPath('LocalApplicationData')
    if (!$taskLocal) { throw 'LocalApplicationData is unavailable' }
    # Wails bindgen's child Go process and Go 1.20 require a workspace temp in
    # this file sandbox. ACL tests in the suite need a private local directory;
    # the build is expected to run with permission to change only test DACLs.
    $taskWorkTemp = Join-Path $taskRoot 'cache/tmp'
    New-Item -ItemType Directory -Force -Path $taskWorkTemp | Out-Null
    $env:TEMP = $taskWorkTemp
    $env:TMP = $taskWorkTemp
    $env:GOTMPDIR = $taskWorkTemp
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
    $env:TEMP = $taskWorkTemp
    $env:TMP = $taskWorkTemp
    $env:GOTMPDIR = $taskWorkTemp
    Invoke-ModernStep go @('test', './...', '-count=1')
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
    # Plain go build does not generate Windows resources, so embed the icon,
    # version resource and manifest from assets/ before linking the executable.
    $infoJSON = Join-Path $taskRoot 'desktop/modern/build/windows/info.json'
    Write-InfoJSON $infoJSON $Version
    Invoke-ModernStep $Wails @('generate', 'syso', '-arch', 'amd64', '-icon', 'assets/windows/icon.ico', '-manifest', 'assets/windows/windeploy.manifest', '-info', $infoJSON, '-out', 'rsrc_windows_amd64.syso')
    $ldflags = "-s -w -X windeploy/internal/buildinfo.Version=$Version -X windeploy/internal/buildinfo.Commit=$Commit"
    $exe = Join-Path $taskRoot 'bin/WinDeploy-Windows-x64.exe'
    Invoke-ModernStep go @('build', '-trimpath', '-buildvcs=false', '-tags', 'production', '-ldflags', ($ldflags + ' -H windowsgui'), '-o', $exe, '.')
    $info = (Get-Item -LiteralPath $exe).VersionInfo
    if ($info.ProductVersion -ne $Version) { throw "EXE ProductVersion '$($info.ProductVersion)' does not match $Version" }
    if (!$info.FileVersion) { throw 'EXE has no file version resource' }
    if ($Diagnostic) {
        # A console subsystem preserves panics from dependency init(), before main/log setup.
        Invoke-ModernStep go @('build', '-trimpath', '-buildvcs=false', '-tags', 'production', '-ldflags', $ldflags, '-o', (Join-Path $taskRoot 'bin/WinDeploy-Diagnostic-x64.exe'), '.')
    }
    Write-Host "Built $exe ($Version, $Commit)"
} finally {
    Pop-Location
    $env:GOCACHE = $taskOldCache
    $env:GOMODCACHE = $taskOldModCache
    $env:GOOS = $taskOldOS
    $env:GOARCH = $taskOldArch
    $env:GOWORK = $taskOldWork
    $env:TEMP = $taskOldTemp
    $env:TMP = $taskOldTmp
    $env:GOTMPDIR = $taskOldGoTmp
}
