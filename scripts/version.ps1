# The product version is the locked core version: the official daemon installs
# an update only if the installer's version is newer than the running core.
. "$PSScriptRoot/env.ps1"
$Lock = Get-Content (Join-Path $ProjectRoot 'upstream.lock') -Raw
$CoreSection = [regex]::Match($Lock, '(?ms)^\[core\]\s*(.*?)(?=^\[|\z)').Groups[1].Value
$ProductVersion = [regex]::Match($CoreSection, 'tag = "v([^"]+)"').Groups[1].Value
if (!$ProductVersion) { throw 'Missing core tag in upstream.lock' }
