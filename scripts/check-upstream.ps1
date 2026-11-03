$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path $PSScriptRoot -Parent
$Lock = Get-Content (Join-Path $ProjectRoot 'upstream.lock') -Raw
foreach ($Item in @(@('core','sing-box'), @('apple_client','sing-box-for-apple'), @('mygo','mygo'))) {
    $Section = [regex]::Match($Lock, '(?ms)^\[' + $Item[0] + '\]\s*(.*?)(?=^\[|\z)').Groups[1].Value
    $Expected = [regex]::Match($Section, 'commit = "([a-f0-9]{40})"').Groups[1].Value
    if (!$Expected) { throw "Missing commit for $($Item[0])" }
    $Source = Join-Path $ProjectRoot ('third_party/' + $Item[1])
    $Actual = & git -C $Source rev-parse HEAD
    if ($LASTEXITCODE -ne 0 -or $Actual -ne $Expected) { throw "Commit mismatch: $Source" }
    $Dirty = & git -C $Source status --porcelain --untracked-files=all
    if ($LASTEXITCODE -ne 0 -or $Dirty) { throw "Modified upstream: $Source" }
    Write-Output "$($Item[0]): $Actual clean"
}
$CoreSection = [regex]::Match($Lock, '(?ms)^\[core\]\s*(.*?)(?=^\[|\z)').Groups[1].Value
$CoreTag = [regex]::Match($CoreSection, 'tag = "(v[0-9]+\.[0-9]+\.[0-9]+(?:-[a-z]+\.[0-9]+)?)"').Groups[1].Value
$CoreCommit = [regex]::Match($CoreSection, 'commit = "([a-f0-9]{40})"').Groups[1].Value
if (!$CoreTag -or !$CoreCommit) { throw 'Invalid core lock' }
$Tag = & git -C (Join-Path $ProjectRoot 'third_party/sing-box') rev-parse "$CoreTag^{commit}"
if ($LASTEXITCODE -ne 0 -or $Tag -ne $CoreCommit) { throw 'Core tag mismatch' }
