param(
    [string]$Version = '',
    [string]$Commit = '',
    [switch]$SkipBuild,
    [string]$Wails = ''
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
if (!$Version) { $Version = (Get-Content -LiteralPath (Join-Path $root 'VERSION') -Raw).Trim() }
if ($Version -notmatch '^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$') {
    throw "Invalid semantic version: $Version"
}
$packageFile = Join-Path $root 'desktop/modern/frontend/package.json'
$package = Get-Content -LiteralPath $packageFile -Raw | ConvertFrom-Json
if ($package.version -ne $Version) { throw "package.json version $($package.version) does not match VERSION $Version" }
$notesSource = Join-Path $root ("docs/RELEASE_NOTES_v{0}.md" -f $Version)
if (!(Test-Path -LiteralPath $notesSource)) { throw "Release notes are missing: $notesSource" }

if (!$SkipBuild) {
    $build = Join-Path $PSScriptRoot 'build-modern.ps1'
    $buildArgs = @('-Version', $Version)
    if ($Commit) { $buildArgs += @('-Commit', $Commit) }
    if ($Wails) { $buildArgs += @('-Wails', $Wails) }
    & $build @buildArgs
    if ($LASTEXITCODE -ne 0) { throw 'Modern build failed' }
}
if (!$Commit) {
    $Commit = (& git -C $root rev-parse --verify HEAD 2>$null)
    if ($LASTEXITCODE -ne 0 -or !$Commit) { $Commit = 'unknown' }
    $Commit = $Commit.Trim()
}

$sourceExe = Join-Path $root 'bin/WinDeploy-Windows-x64.exe'
if (!(Test-Path -LiteralPath $sourceExe)) { throw 'Modern EXE is missing; run scripts/build-modern.ps1 first' }
$info = (Get-Item -LiteralPath $sourceExe).VersionInfo
if ($info.ProductVersion -ne $Version) { throw "EXE version '$($info.ProductVersion)' does not match $Version" }
$signature = Get-AuthenticodeSignature -LiteralPath $sourceExe
$signed = $signature.Status -eq 'Valid'

$dist = Join-Path $root 'dist'
$stage = Join-Path $root ('local/release-stage-v{0}' -f $Version)
$portable = Join-Path $stage 'portable'
New-Item -ItemType Directory -Force -Path $dist, $portable | Out-Null
$artifact = Join-Path $dist 'WinDeploy-Windows-x64.exe'
$zip = Join-Path $dist ("WinDeploy-{0}-portable-x64.zip" -f $Version)
$sums = Join-Path $dist 'SHA256SUMS.txt'
$distNotes = Join-Path $dist 'RELEASE-NOTES.md'
Remove-Item -LiteralPath $artifact, $zip, $sums, $distNotes -Force -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $portable | Out-Null

Copy-Item -LiteralPath $sourceExe -Destination $artifact
Copy-Item -LiteralPath $notesSource -Destination $distNotes
$portableExe = Join-Path $portable 'WinDeploy-Windows-x64.exe'
Copy-Item -LiteralPath $sourceExe -Destination $portableExe
foreach ($file in @('README.md', 'CHANGELOG.md', 'LICENSE', 'CONTRIBUTING.md', 'SECURITY.md')) {
    $path = Join-Path $root $file
    if (Test-Path -LiteralPath $path) { Copy-Item -LiteralPath $path -Destination $portable }
}
$portableDocs = Join-Path $portable 'docs'
Copy-Item -LiteralPath (Join-Path $root 'docs/KNOWN_ISSUES.md') -Destination (Join-Path $portable 'KNOWN_ISSUES.md')
Copy-Item -Recurse -LiteralPath (Join-Path $root 'docs') -Destination $portableDocs
Copy-Item -LiteralPath $notesSource -Destination (Join-Path $portable 'RELEASE-NOTES.md')
# Do not ship local screenshots or caches; the docs folder is source documentation only.
Get-ChildItem -LiteralPath $portableDocs -Recurse -File | Where-Object { $_.Extension -notin @('.md', '.png') } | Remove-Item -Force
$stamp = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$buildInfo = @(
    "WinDeploy $Version",
    "Commit: $Commit",
    "BuiltAtUTC: $stamp",
    "Architecture: windows/amd64",
    "Signature: $(if ($signed) { 'valid' } else { 'NotSigned' })",
    'Runtime: WebView2 Evergreen required; Go/Node.js are not required on the target machine.'
) -join "`r`n"
[IO.File]::WriteAllText((Join-Path $portable 'BUILD-INFO.txt'), $buildInfo + "`r`n", (New-Object System.Text.UTF8Encoding($false)))
Compress-Archive -Path (Join-Path $portable '*') -DestinationPath $zip -CompressionLevel Optimal

$files = @($artifact, $zip, $distNotes)
$hashes = foreach ($file in $files) {
    "{0}  {1}" -f (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant(), (Split-Path -Leaf $file)
}
[IO.File]::WriteAllLines($sums, $hashes, (New-Object System.Text.UTF8Encoding($false)))
foreach ($line in $hashes) {
    $parts = $line -split '  ', 2
    $actual = (Get-FileHash -LiteralPath (Join-Path $dist $parts[1]) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $parts[0]) { throw "SHA-256 verification failed for $($parts[1])" }
}
if (!$signed) {
    Write-Warning 'Code signing certificate is not configured; the EXE is unsigned. This must be stated in release notes.'
}
Write-Host "Modern release artifacts for $Version ($Commit):"
$hashes | ForEach-Object { Write-Host $_ }
Write-Host "Artifacts: $dist"
