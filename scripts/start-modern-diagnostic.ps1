# Run from a PowerShell terminal. This only launches the app; it installs nothing.
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
try {
    $ErrorActionPreference = 'Continue'
    & $taskExe 2>&1 | Tee-Object -FilePath $taskLog
    $taskExit = $LASTEXITCODE
} finally {
    $ErrorActionPreference = $taskPreviousPreference
}
Write-Host "WinDeploy exit code: $taskExit"
if ($taskExit -ne 0) {
    throw "WinDeploy failed before or during startup (exit $taskExit). See $taskLog and %LOCALAPPDATA%\WinDeploy\logs\startup.log. An absent/newly unchanged startup.log means main may not have been reached."
}
