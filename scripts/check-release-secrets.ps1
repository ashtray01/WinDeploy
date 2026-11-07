param(
    [switch]$IncludeHistory
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$blocking = New-Object System.Collections.Generic.List[object]
$review = New-Object System.Collections.Generic.List[object]

function Add-Blocking([string]$Kind, [string]$Where, [string]$Pattern) {
    $blocking.Add([pscustomobject]@{ Kind = $Kind; Where = $Where; Pattern = $Pattern }) | Out-Null
}
function Add-Review([string]$Kind, [string]$Where, [string]$Pattern) {
    $review.Add([pscustomobject]@{ Kind = $Kind; Where = $Where; Pattern = $Pattern }) | Out-Null
}

# High-confidence credential formats. Values are never printed.
$secretPatterns = [ordered]@{
    'private-key' = 'BEGIN (RSA |OPENSSH |EC |DSA |PGP )?PRIVATE KEY'
    'github-token' = '(ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{30,})'
    'slack-token' = 'xox[baprs]-[A-Za-z0-9-]{10,}'
    'aws-access-key' = 'AKIA[0-9A-Z]{16}'
    'openai-style-key' = 'sk-[A-Za-z0-9]{32,}'
    'connection-string-password' = 'Password\s*=\s*[^;"''\s]{8,};'
}

$tracked = @(& git -C $root ls-files)
if ($LASTEXITCODE -ne 0) { throw 'git ls-files failed' }
foreach ($patternName in $secretPatterns.Keys) {
    $pattern = $secretPatterns[$patternName]
    $matches = @(& git -C $root grep -nI -i -E -- $pattern)
    if ($LASTEXITCODE -gt 1) { throw "git grep failed for $patternName" }
    foreach ($match in $matches) {
        if (!$match) { continue }
        $path = ($match -split ':', 2)[0]
        Add-Blocking 'high-confidence-secret-format' $path $patternName
    }
}

# Password-like assignments are meaningful in configuration files only. Go/TS
# test fixtures such as `Password: "stored-password"` are not release secrets.
$configPattern = '(password|passwd|secret|api[_-]?key)\s*[:=]\s*["''][^"'']{8,}["'']'
$configFiles = $tracked | Where-Object { $_ -match '\.(env|ya?ml|json|toml|ini|config|xml|ps1)$' -and $_ -notmatch '(^|/)(docs|local|cache)/' }
foreach ($file in $configFiles) {
    $full = Join-Path $root $file
    if (!(Test-Path -LiteralPath $full)) { continue }
    $matches = @(Select-String -LiteralPath $full -Pattern $configPattern -AllMatches)
    if ($matches.Count -gt 0) { Add-Blocking 'password-like-config-value' $file 'manual-config-review' }
}

# Personal data is not necessarily a secret, but must be reviewed before a public
# source release. Only the file is reported; matching values are never printed.
foreach ($patternName in @('windows-user-path', 'onedrive-path', 'private-ipv4')) {
    $pattern = switch ($patternName) {
        'windows-user-path' { 'C:\\Users\\[A-Za-z0-9._-]+' }
        'onedrive-path' { 'OneDrive[\\/]' }
        'private-ipv4' { '\b(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3})\b' }
    }
    $matches = @(& git -C $root grep -nI -E -- $pattern)
    if ($LASTEXITCODE -gt 1) { throw "git grep failed for $patternName" }
    foreach ($match in $matches) {
        if (!$match) { continue }
        $path = ($match -split ':', 2)[0]
        Add-Review 'manual-review-personal-data' $path $patternName
    }
}

# Files that should never be committed.
foreach ($file in $tracked) {
    if ($file -match '(?i)(\.exe$|\.dll$|\.zip$|\.7z$|\.msi$|\.pdb$|\.log$|(^|/)\.env($|\.)|(^|/)(bin|dist|cache|local|logs)/)') {
        Add-Blocking 'forbidden-artifact' $file 'tracked-runtime-artifact'
    }
    $full = Join-Path $root $file
    if (Test-Path -LiteralPath $full) {
        $size = (Get-Item -LiteralPath $full).Length
        if ($size -gt 5MB) { Add-Review 'large-tracked-file' $file 'greater-than-5MB' }
    }
}

if ($IncludeHistory) {
    $historyFile = Join-Path $root 'cache/tmp/release-history.patch'
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $historyFile) | Out-Null
    & git -C $root log -p --all --no-ext-diff --no-color -- . | Set-Content -LiteralPath $historyFile -Encoding utf8
    if ($LASTEXITCODE -ne 0) { throw 'git log failed' }
    foreach ($patternName in $secretPatterns.Keys) {
        $pattern = $secretPatterns[$patternName]
        $hits = @(Select-String -LiteralPath $historyFile -Pattern $pattern -AllMatches -CaseSensitive:$false)
        foreach ($hit in $hits) {
            Add-Blocking 'history-secret-pattern-review' ("history-line-{0}" -f $hit.LineNumber) $patternName
        }
    }
    Remove-Item -LiteralPath $historyFile -Force -ErrorAction SilentlyContinue
}

$uniqueBlocking = $blocking | Sort-Object Kind, Where, Pattern -Unique
$uniqueReview = $review | Sort-Object Kind, Where, Pattern -Unique
if ($uniqueReview.Count -gt 0) {
    Write-Host 'Manual-review items (no values printed):'
    $uniqueReview | Format-Table -AutoSize
}
if ($uniqueBlocking.Count -eq 0) {
    Write-Host 'Secret preflight: no blocking findings in the selected scope.'
    exit 0
}
Write-Host 'Blocking findings (no values printed):'
$uniqueBlocking | Format-Table -AutoSize
Write-Warning "Secret preflight found $($uniqueBlocking.Count) blocking item(s). Do not open the repository until every item is resolved."
exit 1
