#requires -Version 7.2
# Temporary local M0 volume. Never formats an existing disk or changes C:/D: ACLs.
[CmdletBinding()]
param([ValidateSet('Create', 'Install', 'Detach')][string]$Action = 'Create')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. "$PSScriptRoot/../env.ps1"
$VhdPath = Join-Path $ProjectRoot 'build/m0-test.vhdx'
$StatePath = Join-Path $ProjectRoot 'build/m0-test-volume.json'
$LogPath = Join-Path $ProjectRoot 'build/m0-test-volume.log'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not [Security.Principal.WindowsPrincipal]::new($identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'An elevated PowerShell is required.' }
Start-Transcript -LiteralPath $LogPath -Append | Out-Null
try {
    if ($Action -eq 'Detach') {
        if (Get-Service sing-box-daemon -ErrorAction SilentlyContinue) { throw 'Uninstall the test service before detaching its disk.' }
        if (!(Test-Path -LiteralPath $StatePath)) { throw 'Missing test disk ownership record' }
        $state = Get-Content -LiteralPath $StatePath -Raw | ConvertFrom-Json
        if ($state.path -ne $VhdPath -or $state.purpose -ne 'ProxyGlass-M0') { throw 'Unexpected test disk record' }
        $vhd = Get-VHD -Path $VhdPath
        if ($vhd.DiskIdentifier.ToString() -ne $state.identifier) { throw 'VHD identity changed' }
        Dismount-VHD -Path $VhdPath
        Write-Output 'Detached test VHD; file and evidence retained.'
        return
    }
    if ($Action -eq 'Install') {
        if (!(Test-Path -LiteralPath $StatePath)) { throw 'Missing test disk ownership record' }
        $state = Get-Content -LiteralPath $StatePath -Raw | ConvertFrom-Json
        if ($state.path -ne $VhdPath -or $state.purpose -ne 'ProxyGlass-M0') { throw 'Unexpected test disk record' }
        $vhd = Get-VHD -Path $VhdPath
        if ($vhd.DiskIdentifier.ToString() -ne $state.identifier) { throw 'VHD identity changed' }
        if (!$vhd.Attached) {
            if (Get-PSDrive P -ErrorAction SilentlyContinue) { throw 'P: is occupied by another drive' }
            $vhd = Mount-VHD -Path $VhdPath -PassThru
        }
        $disk = $vhd | Get-Disk
        $partitions = @($disk | Get-Partition | Where-Object DriveLetter -eq 'P')
        if ($partitions.Count -ne 1 -or ($partitions[0] | Get-Volume).FileSystemLabel -ne 'ProxyGlassM0') { throw 'Expected P: on the recorded test disk; assign it manually in Disk Management if needed.' }
        # The installation preflight below checks NTFS, root ACL and conflicts.
        # Existing test disks are never initialized or formatted by this action.
    } else {
    if ((Test-Path -LiteralPath $VhdPath) -or (Test-Path -LiteralPath $StatePath) -or (Get-PSDrive P -ErrorAction SilentlyContinue)) { throw 'Test file, record or P: already exists; refusing to reuse.' }
    if ((Get-Service sing-box-daemon -ErrorAction SilentlyContinue) -or (Test-Path 'HKLM:\SOFTWARE\SagerNet\sing-box')) { throw 'Existing sing-box resources; refusing to create test installation.' }
    $vhd = New-VHD -Path $VhdPath -Dynamic -SizeBytes 2GB
    [ordered]@{purpose='ProxyGlass-M0';path=$VhdPath;identifier=$vhd.DiskIdentifier.ToString();created=(Get-Date).ToUniversalTime().ToString('o')} |
        ConvertTo-Json | Set-Content -LiteralPath $StatePath -Encoding utf8
    $mounted = Mount-VHD -Path $VhdPath -PassThru
    $disk = $mounted | Get-Disk
    if ($disk.PartitionStyle -ne 'RAW' -or $disk.Size -ne 2GB -or $disk.IsBoot -or $disk.IsSystem -or @($disk | Get-Partition -ErrorAction SilentlyContinue).Count -ne 0) { throw 'New virtual disk did not match an empty non-system RAW disk.' }
    $disk | Initialize-Disk -PartitionStyle GPT | Out-Null
    $partition = $disk | New-Partition -UseMaximumSize -DriveLetter P
    if ($partition.DiskNumber -ne $disk.Number -or $partition.DriveLetter -ne 'P') { throw 'Partition identity mismatch' }
    $partition | Format-Volume -FileSystem NTFS -NewFileSystemLabel ProxyGlassM0 -Confirm:$false | Out-Null
    $acl = [Security.AccessControl.DirectorySecurity]::new()
    $acl.SetSecurityDescriptorSddlForm('O:BAG:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GRGX;;;AU)')
    Set-Acl -LiteralPath 'P:\' -AclObject $acl
    Write-Output 'Created P: on the newly created test VHD; root permissions are protected.'
    }
    & "$PSScriptRoot/manage.ps1" -Action Install -Source (Join-Path $ProjectRoot 'build/app') -Certificate (Join-Path $ProjectRoot 'build/cert-m0-service/codesign.cer') -InstallDirectory 'P:\ProxyGlassDev' -DataDirectory 'P:\ProxyGlassDevData' -Plan
    & "$PSScriptRoot/manage.ps1" -Action Install -Source (Join-Path $ProjectRoot 'build/app') -Certificate (Join-Path $ProjectRoot 'build/cert-m0-service/codesign.cer') -InstallDirectory 'P:\ProxyGlassDev' -DataDirectory 'P:\ProxyGlassDevData' -Confirm:$false
    Write-Output 'Installed the official test daemon. Run probes from the normal interactive user session.'
} catch {
    $_ | Out-String | Set-Content -LiteralPath (Join-Path $ProjectRoot 'build/m0-test-volume-error.txt') -Encoding utf8
    throw
} finally { Stop-Transcript | Out-Null }
