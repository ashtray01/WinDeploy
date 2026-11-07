param([string]$Version = '')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$versionFile = Join-Path $root 'VERSION'
if (!(Test-Path -LiteralPath $versionFile)) { throw 'VERSION is missing' }
$canonical = (Get-Content -LiteralPath $versionFile -Raw).Trim()
if ($Version -and $Version -ne $canonical) { throw "Requested version $Version does not match VERSION $canonical" }
if ($canonical -notmatch '^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$') {
    throw "VERSION is not semantic: $canonical"
}
$package = Get-Content -LiteralPath (Join-Path $root 'desktop/modern/frontend/package.json') -Raw | ConvertFrom-Json
if ($package.version -ne $canonical) { throw "package.json $($package.version) != VERSION $canonical" }
$lock = Get-Content -LiteralPath (Join-Path $root 'desktop/modern/frontend/package-lock.json') -Raw | ConvertFrom-Json -AsHashtable
$lockRoot = $lock.packages['']
if ($lock.version -ne $canonical -or $lockRoot.version -ne $canonical) { throw "package-lock is not $canonical" }
$changelog = Get-Content -LiteralPath (Join-Path $root 'CHANGELOG.md') -Raw
if ($changelog -notmatch [regex]::Escape("## [$canonical]")) { throw "CHANGELOG has no entry for $canonical" }
$notes = Join-Path $root ("docs/RELEASE_NOTES_v{0}.md" -f $canonical)
if (!(Test-Path -LiteralPath $notes)) { throw "Release notes missing: $notes" }
Write-Host "Version identity is consistent: $canonical"
