# Builds the official daemon and the application at the locked versions, signs
# both with the development certificate and packs the NSIS installer.
#   ./scripts/package.ps1 -Pfx build/cert-m0-service/codesign.pfx -Certificate build/cert-m0-service/codesign.cer -Password $pw
# Without -Pfx nothing is signed (a local dry run; the core rejects such an update).
[CmdletBinding()]
param(
    [string]$Pfx,
    [string]$Certificate,
    [System.Security.SecureString]$Password,
    [string]$MakeNSIS,
    # Development: label a test installer newer than the installed core
    # (as 1.14.5-alpha.1) without moving the core lock.
    [string]$InstallerVersion,
    [switch]$SkipBuild
)
. "$PSScriptRoot/version.ps1"
if (!$SkipBuild) {
    & "$PSScriptRoot/daemon/build.ps1" -Arch amd64
    & "$PSScriptRoot/build.ps1"
}
if ($InstallerVersion) { $ProductVersion = $InstallerVersion }
$App = Join-Path $ProjectRoot 'build/app'
if (!$SkipBuild) {
    # -SkipBuild repacks the already signed build/app as it is.
    $Daemon = Join-Path $App 'resources/daemon'
    New-Item -ItemType Directory -Force $Daemon | Out-Null
    Copy-Item -Path (Join-Path $ProjectRoot 'build/amd64/resources/daemon/*') -Destination $Daemon -Recurse -Force
}
$Executables = @('build/app/sing-box.exe', 'build/app/resources/daemon/sing-box-daemon.exe')
$Sign = {
    param([string[]]$Files)
    if (!$Pfx) { return }
    $arguments = @{ Pfx = $Pfx; Certificate = $Certificate; Files = $Files }
    if ($Password) { $arguments.Password = $Password }
    & "$PSScriptRoot/sign/sign.ps1" @arguments
}
if (!$SkipBuild) { & $Sign $Executables }

if (!$MakeNSIS) {
    $command = Get-Command makensis.exe -ErrorAction SilentlyContinue
    $MakeNSIS = if ($command) { $command.Source } else { Join-Path ${env:ProgramFiles(x86)} 'NSIS/makensis.exe' }
}
if (!(Test-Path -LiteralPath $MakeNSIS)) { throw 'makensis.exe not found (winget install NSIS.NSIS)' }
$Numeric = [regex]::Match($ProductVersion, '^(\d+)\.(\d+)\.(\d+)')
$Version4 = '{0}.{1}.{2}.0' -f $Numeric.Groups[1].Value, $Numeric.Groups[2].Value, $Numeric.Groups[3].Value
$Out = Join-Path $ProjectRoot 'build/release'
New-Item -ItemType Directory -Force $Out | Out-Null
$Installer = Join-Path $Out "sing-box-for-windows-$ProductVersion-windows-amd64-setup.exe"
& $MakeNSIS /V2 "/DVERSION=$ProductVersion" "/DVERSION4=$Version4" "/DSOURCE=$App" "/DOUTFILE=$Installer" (Join-Path $ProjectRoot 'installer/sing-box.nsi')
if ($LASTEXITCODE -ne 0) { throw 'makensis failed' }
& $Sign @($Installer)
$Hash = (Get-FileHash -LiteralPath $Installer -Algorithm SHA256).Hash.ToLowerInvariant()
"$Hash  $(Split-Path $Installer -Leaf)" | Set-Content -Encoding ascii (Join-Path $Out 'SHA256SUMS')
Write-Output "Installer: $Installer"
Write-Output "SHA256: $Hash"
