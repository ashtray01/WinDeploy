# Run from a PowerShell terminal. This only launches the app; it installs nothing.
param([string]$WebViewDataDir = '', [int]$SmokePort = 0, [string]$StartupLogDir = '')
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskExe = Join-Path $taskRoot 'bin/WinDeploy-Diagnostic-x64.exe'
if (!(Test-Path -LiteralPath $taskExe)) {
    throw 'Build first: .\scripts\build-modern.ps1 -Diagnostic'
}
$taskLogDir = Join-Path $taskRoot 'local'
New-Item -ItemType Directory -Force -Path $taskLogDir | Out-Null
$taskLog = Join-Path $taskLogDir 'modern-diagnostic.log'
Write-Host "Diagnostic output: $taskLog"
# Native stderr must remain observable even for dependency init() failures.
$taskPreviousPreference = $ErrorActionPreference
$taskPreviousConsoleEncoding = [Console]::OutputEncoding
$taskPreviousOutputEncoding = $OutputEncoding
$taskUtf8 = [Text.UTF8Encoding]::new($false)
$taskLogWriter = [IO.StreamWriter]::new($taskLog, $false, $taskUtf8)
$taskLogWriter.AutoFlush = $true
if (!$StartupLogDir) { $StartupLogDir = Join-Path $taskLogDir 'startup' }
try {
    # Go writes UTF-8. Windows PowerShell may otherwise decode stderr using OEM codepage.
    [Console]::OutputEncoding = $taskUtf8
    $OutputEncoding = $taskUtf8
    $ErrorActionPreference = 'Continue'
    $taskArguments = @('--log-dir', $StartupLogDir)
    if ($WebViewDataDir) { $taskArguments += @('--webview-data-dir', $WebViewDataDir) }
    if ($SmokePort) { $taskArguments += @('--smoke-port', "$SmokePort") }
    & $taskExe @taskArguments 2>&1 | ForEach-Object {
        $taskLogWriter.WriteLine($_.ToString())
        Write-Output $_
    }
    $taskExit = $LASTEXITCODE
} finally {
    $taskLogWriter.Dispose()
    [Console]::OutputEncoding = $taskPreviousConsoleEncoding
    $OutputEncoding = $taskPreviousOutputEncoding
    $ErrorActionPreference = $taskPreviousPreference
}
Write-Host "WinDeploy exit code: $taskExit"
if ($taskExit -ne 0) {
    throw "WinDeploy failed before or during startup (exit $taskExit). See $taskLog and %LOCALAPPDATA%\WinDeploy\logs\startup.log. The file log may be unavailable; console output contains the current startup evidence."
}
