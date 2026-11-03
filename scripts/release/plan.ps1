# Decides whether the release workflow builds: the latest stable official
# sing-box core is locked (upstream.lock and the third_party/sing-box
# submodule move to it when it is newer), and a release is built when this
# repository has none for that version yet. Writes build/version/bumped to
# $env:GITHUB_OUTPUT.
. "$PSScriptRoot/../env.ps1"
$LockPath = Join-Path $ProjectRoot 'upstream.lock'
$Lock = Get-Content $LockPath -Raw
$CoreSection = [regex]::Match($Lock, '(?ms)^\[core\]\s*(.*?)(?=^\[|\z)').Groups[1].Value
$Locked = [regex]::Match($CoreSection, 'tag = "(v[^"]+)"').Groups[1].Value
if (!$Locked) { throw 'Missing core tag in upstream.lock' }
$Core = Join-Path $ProjectRoot 'third_party/sing-box'

$Official = & gh api repos/SagerNet/sing-box/releases/latest --jq .tag_name
if ($LASTEXITCODE -ne 0 -or $Official -notmatch '^v\d+\.\d+\.\d+$') { throw "Cannot read the latest official release ($Official)" }

function Newer([string]$a, [string]$b) { return [version]($a.TrimStart('v')) -gt [version]($b.TrimStart('v')) }

$Tag = $Locked
$Bumped = $false
if ($Locked -match '^v\d+\.\d+\.\d+$' -and (Newer $Official $Locked)) { $Tag = $Official; $Bumped = $true }
& git -C $Core fetch --no-tags --depth=1 origin "refs/tags/${Tag}:refs/tags/${Tag}"
if ($LASTEXITCODE -ne 0) { throw "Cannot fetch core tag $Tag" }
if ($Bumped) {
    & git -C $Core checkout --quiet --detach "$Tag^{commit}"
    if ($LASTEXITCODE -ne 0) { throw "Cannot check out core $Tag" }
    $Commit = & git -C $Core rev-parse "$Tag^{commit}"
    $NewSection = $CoreSection -replace 'tag = "v[^"]+"', "tag = `"$Tag`"" -replace 'commit = "[a-f0-9]{40}"', "commit = `"$Commit`""
    $Lock.Replace($CoreSection, $NewSection) | Set-Content -NoNewline -Encoding utf8NoBOM $LockPath
    Write-Output "Core $Locked -> $Tag ($Commit)"
}

$Version = $Tag.TrimStart('v')
& gh release view "v$Version" --json tagName 2>$null | Out-Null
$Exists = $LASTEXITCODE -eq 0
$global:LASTEXITCODE = 0
$Build = -not $Exists
Write-Output "Official $Official, locked $Locked, release v$Version exists: $Exists"
if ($env:GITHUB_OUTPUT) {
    "build=$($Build.ToString().ToLower())" >> $env:GITHUB_OUTPUT
    "version=$Version" >> $env:GITHUB_OUTPUT
    "bumped=$($Bumped.ToString().ToLower())" >> $env:GITHUB_OUTPUT
}
