#requires -Version 7.2
# Repair/uninstall only the marker-owned installation, then detach its recorded VHD.
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/../env.ps1"
$Certificate = Join-Path $ProjectRoot 'build/cert-m0-service/codesign.cer'
Start-Transcript -LiteralPath (Join-Path $ProjectRoot 'build/m0-test-cleanup.log') -Append | Out-Null
try {
    & "$PSScriptRoot/manage.ps1" -Action Repair -Certificate $Certificate -InstallDirectory 'P:\ProxyGlassDev' -DataDirectory 'P:\ProxyGlassDevData' -Confirm:$false
    & "$PSScriptRoot/manage.ps1" -Action Uninstall -Certificate $Certificate -InstallDirectory 'P:\ProxyGlassDev' -DataDirectory 'P:\ProxyGlassDevData' -Confirm:$false
    & "$PSScriptRoot/test-volume.ps1" -Action Detach
} catch {
    $_ | Out-String | Set-Content -LiteralPath (Join-Path $ProjectRoot 'build/m0-test-cleanup-error.txt') -Encoding utf8
    throw
} finally { Stop-Transcript | Out-Null }
