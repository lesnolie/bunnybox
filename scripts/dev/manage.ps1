#requires -Version 7.2
[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Medium')]
param(
    [Parameter(Mandatory)][ValidateSet('Install', 'Repair', 'Uninstall')][string]$Action,
    [string]$Source,
    [Parameter(Mandatory)][string]$Certificate,
    [string]$InstallDirectory = 'C:\Program Files\ProxyGlassDev',
    [string]$DataDirectory = 'C:\ProgramData\ProxyGlassDev',
    [string]$EvidenceDirectory,
    [switch]$Plan
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows -or -not [Environment]::Is64BitProcess) { throw 'Run in 64-bit PowerShell 7 on Windows.' }
. (Join-Path $PSScriptRoot '../env.ps1')
$product = 'ProxyGlassDev/7ac8d978-3e72-4bdb-8bf8-d8b10965e2b1'
$registryPath = 'SOFTWARE\SagerNet\sing-box'
$markerName = '.proxyglass-dev-install.json'
$trusted = @('S-1-5-18', 'S-1-5-32-544', 'S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464')

function Full-Path([string]$Value) {
    if ($Value -notmatch '^[A-Za-z]:\\' -or $Value.Substring(2).Contains(':') -or $Value -match '[*?]') { throw "An absolute local drive path is required: $Value" }
    $result = [IO.Path]::GetFullPath($Value).TrimEnd('\')
    if ($result.Length -le 3 -or $result -cne $Value.TrimEnd('\')) { throw "Use a canonical non-root path without dot segments: $Value" }
    return $result
}
function Is-Within([string]$Child, [string]$Parent) {
    return $Child.Equals($Parent, [StringComparison]::OrdinalIgnoreCase) -or $Child.StartsWith($Parent + '\', [StringComparison]::OrdinalIgnoreCase)
}
function Assert-Separate([string]$A, [string]$B) {
    if ((Is-Within $A $B) -or (Is-Within $B $A)) { throw "Directories must not overlap: $A / $B" }
}
function Assert-NoReparse([string]$Path, [switch]$Tree) {
    $current = $Path
    while ($current) {
        if (Test-Path -LiteralPath $current) {
            $item = Get-Item -LiteralPath $current -Force
            if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Reparse point rejected: $current" }
        }
        $parent = Split-Path $current -Parent
        if (-not $parent -or $parent -eq $current) { break }; $current = $parent
    }
    if ($Tree -and (Test-Path -LiteralPath $Path)) {
        # Enumerate one level at a time; never descend through a reparse point.
        foreach ($item in Get-ChildItem -LiteralPath $Path -Force) {
            if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Reparse point rejected: $($item.FullName)" }
            if ($item.PSIsContainer) { Assert-NoReparse $item.FullName -Tree }
        }
    }
}
function Assert-Ancestors([string]$Path) {
    $drive = [IO.DriveInfo]::new([IO.Path]::GetPathRoot($Path))
    if ($drive.DriveType -ne [IO.DriveType]::Fixed -or $drive.DriveFormat -ne 'NTFS') { throw "A fixed local NTFS volume is required: $Path" }
    Assert-NoReparse $Path
    $current = Split-Path $Path -Parent
    while ($current) {
        if (-not (Test-Path -LiteralPath $current -PathType Container)) { throw "Parent directory must already exist: $current" }
        $acl = Get-Acl -LiteralPath $current
        if ($trusted -notcontains $acl.GetOwner([Security.Principal.SecurityIdentifier]).Value) { throw "Untrusted ancestor owner: $current" }
        $raw = [Security.AccessControl.RawSecurityDescriptor]::new($acl.GetSecurityDescriptorBinaryForm(), 0)
        if ($null -eq $raw.DiscretionaryAcl) { throw "Null ancestor DACL: $current" }
        foreach ($ace in $raw.DiscretionaryAcl) {
            if ($ace -isnot [Security.AccessControl.CommonAce]) { throw "Unsupported ancestor ACE: $current" }
            if ($ace.AceFlags -band [Security.AccessControl.AceFlags]::InheritOnly) { continue }
            if ($ace.AceQualifier -eq [Security.AccessControl.AceQualifier]::AccessAllowed -and ($ace.AccessMask -band 0x500D0040) -ne 0 -and $trusted -notcontains $ace.SecurityIdentifier.Value) { throw "Ancestor can be replaced by a non-administrator: $current" }
        }
        $parent = Split-Path $current -Parent
        if (-not $parent -or $parent -eq $current) { break }; $current = $parent
    }
}
function Inventory([string]$Root) {
    Assert-NoReparse $Root -Tree
    $result = @()
    foreach ($file in Get-ChildItem -LiteralPath $Root -File -Recurse -Force) {
        $relative = [IO.Path]::GetRelativePath($Root, $file.FullName)
        if ($relative -eq $markerName) { continue }
        if ($file.Extension -in '.pfx', '.p12', '.pem', '.key') { throw "Private-key-like files are not installation payload: $relative" }
        $result += [pscustomobject]@{ Path = $relative; SHA256 = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash }
    }
    return @($result | Sort-Object Path)
}
function Assert-Inventory([string]$Root, $Files) {
    Assert-NoReparse $Root -Tree
    foreach ($file in $Files) {
        $target = Full-Path (Join-Path $Root $file.Path)
        if (-not (Is-Within $target $Root) -or $target -eq $Root) { throw 'Invalid marker inventory path.' }
        if (-not (Test-Path -LiteralPath $target -PathType Leaf) -or (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $file.SHA256) { throw "Payload missing or modified; manual review required: $target" }
    }
}
function Assert-OwnedTree([string]$Root) {
    Assert-NoReparse $Root -Tree
    $items = @((Get-Item -LiteralPath $Root -Force)) + @(Get-ChildItem -LiteralPath $Root -Recurse -Force)
    foreach ($item in $items) {
        $acl = Get-Acl -LiteralPath $item.FullName
        if ($trusted -notcontains $acl.GetOwner([Security.Principal.SecurityIdentifier]).Value) { throw "Untrusted installation owner: $($item.FullName)" }
        $raw = [Security.AccessControl.RawSecurityDescriptor]::new($acl.GetSecurityDescriptorBinaryForm(), 0)
        if ($null -eq $raw.DiscretionaryAcl) { throw "Null installation DACL: $($item.FullName)" }
        foreach ($ace in $raw.DiscretionaryAcl) {
            if ($ace -isnot [Security.AccessControl.CommonAce]) { throw "Unsupported installation ACE: $($item.FullName)" }
            if ($ace.AceFlags -band [Security.AccessControl.AceFlags]::InheritOnly) { continue }
            if ($ace.AceQualifier -eq [Security.AccessControl.AceQualifier]::AccessAllowed -and ($ace.AccessMask -band 0x500D0156) -ne 0 -and $trusted -notcontains $ace.SecurityIdentifier.Value) { throw "Installation is writable by a non-administrator: $($item.FullName)" }
        }
    }
}
function Assert-Signatures([string]$Root) {
    $gui = Join-Path $Root 'sing-box.exe'
    $daemon = Join-Path $Root 'resources\daemon\sing-box-daemon.exe'
    foreach ($p in @($gui, $daemon)) { if (-not (Test-Path -LiteralPath $p -PathType Leaf)) { throw "Required signed executable missing: $p" } }
    # This verifier follows the official WinVerifyTrust policy, including valid
    # self-signed development certificates, and compares the exact signer DER.
    & go run (Join-Path $ProjectRoot 'scripts/sign/verify.go') -cert $Certificate $gui $daemon
    if ($LASTEXITCODE -ne 0) { throw 'GUI/daemon signature verification failed.' }
}
function Is-RegistryCreatorTemplate($Ace) {
    # CREATOR OWNER/GROUP are inheritance placeholders, not groups granting
    # current access. Each existing registry key's real owner is checked above;
    # a new key is created only by the elevated installer. Do not apply this
    # exception to the official filesystem ancestor checks.
    return $Ace -is [Security.AccessControl.CommonAce] -and
        $Ace.SecurityIdentifier.Value -in @('S-1-3-0', 'S-1-3-1') -and
        ($Ace.AceFlags -band [Security.AccessControl.AceFlags]::ContainerInherit) -ne 0
}
function Read-Layout {
    foreach ($path in @('SOFTWARE', 'SOFTWARE\SagerNet')) {
        $providerPath = 'Registry::HKEY_LOCAL_MACHINE\' + $path
        if (-not (Test-Path -LiteralPath $providerPath)) { continue }
        $acl = Get-Acl -LiteralPath $providerPath
        if ($trusted -notcontains $acl.GetOwner([Security.Principal.SecurityIdentifier]).Value) { throw "Untrusted registry ancestor owner: $path" }
        $raw = [Security.AccessControl.RawSecurityDescriptor]::new($acl.GetSecurityDescriptorBinaryForm(), 0)
        if ($null -eq $raw.DiscretionaryAcl) { throw "Null registry ancestor DACL: $path" }
        foreach ($ace in $raw.DiscretionaryAcl) {
            if ($ace -isnot [Security.AccessControl.CommonAce]) { throw "Unsupported registry ancestor ACE: $path" }
            if ($ace.AceFlags -band [Security.AccessControl.AceFlags]::InheritOnly) { continue }
            if (Is-RegistryCreatorTemplate $ace) { continue }
            if ($ace.AceQualifier -eq [Security.AccessControl.AceQualifier]::AccessAllowed -and ($ace.AccessMask -band 0x500D0006) -ne 0 -and $trusted -notcontains $ace.SecurityIdentifier.Value) { throw "Registry ancestor is writable by a non-administrator: $path" }
        }
    }
    $base = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::LocalMachine, [Microsoft.Win32.RegistryView]::Registry64)
    try {
        $key = $base.OpenSubKey($registryPath)
        if ($null -eq $key) { return $null }
        try {
            $acl = Get-Acl -LiteralPath ('Registry::HKEY_LOCAL_MACHINE\' + $registryPath)
            if ($trusted -notcontains $acl.GetOwner([Security.Principal.SecurityIdentifier]).Value) { throw 'Untrusted shared layout registry owner.' }
            $raw = [Security.AccessControl.RawSecurityDescriptor]::new($acl.GetSecurityDescriptorBinaryForm(), 0)
            if ($null -eq $raw.DiscretionaryAcl) { throw 'Null shared layout registry DACL.' }
            foreach ($ace in $raw.DiscretionaryAcl) {
                if ($ace -isnot [Security.AccessControl.CommonAce]) { throw 'Unsupported shared layout registry ACE.' }
                if ($ace.AceFlags -band [Security.AccessControl.AceFlags]::InheritOnly) { continue }
                if (Is-RegistryCreatorTemplate $ace) { continue }
                if ($ace.AceQualifier -eq [Security.AccessControl.AceQualifier]::AccessAllowed -and ($ace.AccessMask -band 0x500D0006) -ne 0 -and $trusted -notcontains $ace.SecurityIdentifier.Value) { throw 'Shared layout registry is writable by a non-administrator.' }
            }
            $values = [ordered]@{}; foreach ($name in $key.GetValueNames()) { $values[$name] = $key.GetValue($name) }; return $values
        }
        finally { $key.Dispose() }
    } finally { $base.Dispose() }
}
function Assert-Layout($Layout, $Marker) {
    if ($null -eq $Layout) { return }
    if ($null -eq $Marker -or $Layout['InstallationID'] -ne $Marker.InstallationID -or $Layout['ProxyGlassDevOwner'] -ne $product -or $Layout['ProxyGlassDevInstallDirectory'] -ne $InstallDirectory -or $Layout['LayoutVersion'] -ne 2 -or $Layout['DaemonDataDirectory'] -ne $DataDirectory -or $Layout['ApplicationDataDirectory']) { throw 'Shared official sing-box registry layout is occupied by another installation or is inconsistent. Nothing will be overwritten.' }
}
function Read-Service {
    $service = $null
    try { $service = Get-Service -Name 'sing-box-daemon' -ErrorAction Stop }
    catch { if ($_.FullyQualifiedErrorId -notlike 'NoServiceFoundForGivenName*') { throw } }
    $base = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::LocalMachine, [Microsoft.Win32.RegistryView]::Registry64)
    try {
        $key = $base.OpenSubKey('SYSTEM\CurrentControlSet\Services\sing-box-daemon')
        if ($null -eq $key) { if ($service) { throw 'Service exists but its configuration cannot be read.' }; return $null }
        try {
            if (-not $service) { throw 'Orphaned shared service registry key exists; manual review required.' }
            $startMode = if ($key.GetValue('Start') -eq 2) { 'Auto' } else { 'Other' }
            return [pscustomobject]@{ Name = 'sing-box-daemon'; PathName = $key.GetValue('ImagePath'); StartName = $key.GetValue('ObjectName'); StartMode = $startMode; State = $service.Status.ToString() }
        } finally { $key.Dispose() }
    } finally { $base.Dispose(); if ($service) { $service.Dispose() } }
}
function Assert-Service($Service, $Marker) {
    if ($null -eq $Service) { return }
    $exe = [regex]::Escape((Join-Path $InstallDirectory 'resources\daemon\sing-box-daemon.exe'))
    $data = [regex]::Escape($DataDirectory)
    $expected = '^(?:"' + $exe + '"|' + $exe + ')\s+run\s+--working-directory\s+(?:"' + $data + '"|' + $data + ')$'
    if ($null -eq $Marker -or $Service.PathName -notmatch $expected -or $Service.StartName -notin 'LocalSystem', 'NT AUTHORITY\SYSTEM') { throw 'Shared official sing-box-daemon service belongs to another or inconsistent installation. Nothing will be replaced.' }
}
function Assert-DataRoot {
    if (-not (Test-Path -LiteralPath $DataDirectory)) { return }
    Assert-NoReparse $DataDirectory -Tree
    $acl = Get-Acl -LiteralPath $DataDirectory
    $hash = [Security.Cryptography.SHA1]::HashData([Text.Encoding]::Unicode.GetBytes('SING-BOX-DAEMON'))
    $serviceSid = 'S-1-5-80-' + ((0..4 | ForEach-Object { [BitConverter]::ToUInt32($hash, $_ * 4) }) -join '-')
    $allowed = @('S-1-5-18', 'S-1-5-32-544', $serviceSid)
    $raw = [Security.AccessControl.RawSecurityDescriptor]::new($acl.GetSecurityDescriptorBinaryForm(), 0)
    if ($raw.Owner.Value -ne 'S-1-5-18' -or -not $acl.AreAccessRulesProtected -or $null -eq $raw.DiscretionaryAcl -or $raw.DiscretionaryAcl.Count -ne 3) { throw 'Existing daemon data ACL is not the official protected layout; refusing repair or replacement.' }
    foreach ($ace in $raw.DiscretionaryAcl) {
        if ($ace -isnot [Security.AccessControl.CommonAce] -or $ace.AceQualifier -ne [Security.AccessControl.AceQualifier]::AccessAllowed -or $allowed -notcontains $ace.SecurityIdentifier.Value -or $ace.AccessMask -ne 0x1F01FF -or [int]$ace.AceFlags -ne 3) { throw 'Existing daemon data contains unexpected permissions.' }
        $allowed = @($allowed | Where-Object { $_ -ne $ace.SecurityIdentifier.Value })
    }
    if ($allowed.Count) { throw 'Existing daemon data is missing a required principal.' }
}
function Write-Marker($Marker) {
    $temp = $markerPath + '.new'
    $Marker | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $temp -Encoding utf8NoBOM
    Move-Item -LiteralPath $temp -Destination $markerPath -Force
}
function Official-Command([string[]]$Arguments) {
    $psi = [Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = Join-Path $InstallDirectory 'resources\daemon\sing-box-daemon.exe'
    $psi.UseShellExecute = $false; $psi.CreateNoWindow = $true
    $psi.RedirectStandardOutput = $true; $psi.RedirectStandardError = $true
    foreach ($argument in $Arguments) { $psi.ArgumentList.Add($argument) }
    $process = [Diagnostics.Process]::new(); $process.StartInfo = $psi
    try {
        if (-not $process.Start()) { throw 'Unable to start official daemon command.' }
        [pscustomobject]@{ Executable = $psi.FileName; Arguments = $Arguments; ProcessID = $process.Id; StartedUTC = [DateTime]::UtcNow.ToString('O') } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $evidence 'official-command-started.json') -Encoding utf8NoBOM
        $stdout = $process.StandardOutput.ReadToEndAsync(); $stderr = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit(120000)) { throw 'Official command timed out; process/service state is uncertain. No automatic rollback.' }
        $text = $stdout.GetAwaiter().GetResult() + $stderr.GetAwaiter().GetResult()
        $text | Set-Content -LiteralPath (Join-Path $evidence 'official-command.log') -Encoding utf8NoBOM
        if ($process.ExitCode -ne 0) { throw "Official daemon command failed with exit code $($process.ExitCode). See $evidence" }
    } finally { $process.Dispose() }
}

$InstallDirectory = Full-Path $InstallDirectory
$DataDirectory = Full-Path $DataDirectory
Assert-Separate $InstallDirectory $DataDirectory
Assert-Ancestors $InstallDirectory; Assert-Ancestors $DataDirectory
Assert-NoReparse $InstallDirectory -Tree
$Certificate = (Resolve-Path -LiteralPath $Certificate).Path
$certificateHash = (Get-FileHash -LiteralPath $Certificate -Algorithm SHA256).Hash
$markerPath = Join-Path $InstallDirectory $markerName
$marker = $null
if (Test-Path -LiteralPath $markerPath -PathType Leaf) {
    Assert-OwnedTree $InstallDirectory
    $marker = Get-Content -LiteralPath $markerPath -Raw | ConvertFrom-Json
    $parsedID = [guid]::Empty
    if ($marker.Schema -ne 1 -or $marker.Product -ne $product -or $marker.InstallDirectory -ne $InstallDirectory -or $marker.DataDirectory -ne $DataDirectory -or $marker.CertificateSHA256 -ne $certificateHash -or -not [guid]::TryParse($marker.InstallationID, [ref]$parsedID)) { throw 'Installation marker does not identify this exact installation and signer.' }
} elseif (Test-Path -LiteralPath $InstallDirectory) { throw 'Existing installation path has no matching development marker; refusing to adopt it.' }
if ((Test-Path -LiteralPath $DataDirectory) -and $null -eq $marker) { throw 'Existing daemon data directory has no matching development installation marker; refusing to adopt or delete it.' }
$layout = Read-Layout; Assert-Layout $layout $marker
$service = Read-Service; Assert-Service $service $marker
foreach ($knownRoot in @((Join-Path $env:ProgramFiles 'sing-box'), (Join-Path ${env:ProgramFiles(x86)} 'sing-box'))) {
    if ($knownRoot -ne $InstallDirectory -and (Test-Path -LiteralPath (Join-Path $knownRoot 'sing-box.exe'))) { throw "A different official-layout GUI exists at $knownRoot; review/uninstall that installation first." }
}
if ($DataDirectory -ne 'C:\ProgramData\sing-box-daemon' -and (Test-Path -LiteralPath 'C:\ProgramData\sing-box-daemon')) { throw 'The official default daemon data directory already exists; review the prior installation instead of adopting its resources.' }
if (-not $marker -and (Test-Path -LiteralPath 'HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\sing-box-daemon')) { throw 'An unowned official daemon event-log registration already exists; manual review required.' }
if ($marker -and $marker.State -notin 'Installed', 'Uninstalled') { throw "Previous operation is incomplete ($($marker.State)); preserve evidence and review manually." }
if ($marker) { Assert-DataRoot }
if ($Action -eq 'Install') {
    if ($marker -and $marker.State -ne 'Uninstalled') { throw 'Already installed; use Repair. Replacing signed payload requires an explicit uninstall/install cycle.' }
    if ($service) { throw 'Install requires the prior development service to be uninstalled.' }
    if (-not $Source) { throw 'Install requires -Source with the complete signed installation layout.' }
    $Source = (Resolve-Path -LiteralPath $Source).Path
    Assert-Separate $Source $InstallDirectory; Assert-Separate $Source $DataDirectory
    $files = @(Inventory $Source)
    Assert-Signatures $Source
    foreach ($file in $files) {
        $target = Join-Path $InstallDirectory $file.Path
        if (Test-Path -LiteralPath $target) { throw "Existing file/path conflicts with the new payload: $target" }
        $parent = Split-Path $target -Parent
        while ($parent -ne $InstallDirectory -and (Is-Within $parent $InstallDirectory)) {
            if ((Test-Path -LiteralPath $parent) -and -not (Test-Path -LiteralPath $parent -PathType Container)) { throw "Payload parent is occupied by a file: $parent" }
            $parent = Split-Path $parent -Parent
        }
    }
} else {
    if (-not $marker -or $marker.State -ne 'Installed') { throw "$Action requires an Installed marker owned by this script." }
    $files = @($marker.Files)
    Assert-Inventory $InstallDirectory $files
    Assert-Signatures $InstallDirectory
}
foreach ($process in @(Get-Process | Where-Object ProcessName -eq 'sing-box')) {
    if (-not $process.Path -or $process.Path -eq (Join-Path $InstallDirectory 'sing-box.exe')) { throw 'Close the installed GUI before managing the development installation (or run elevated to identify its path).' }
}
$summary = [pscustomobject]@{ Action = $Action; InstallDirectory = $InstallDirectory; DaemonDataDirectory = $DataDirectory; ServiceName = 'sing-box-daemon'; ExistingService = [bool]$service; ExistingLayout = [bool]$layout; PayloadFiles = $files.Count; KeepData = $true; AutomaticRollback = $false }
$summary | Format-List | Out-Host
if ($Plan -or -not $PSCmdlet.ShouldProcess($InstallDirectory, "$Action owned ProxyGlass development installation; retain all data")) { return }
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not [Security.Principal.WindowsPrincipal]::new($identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Execute mutations from an elevated PowerShell. Plan and WhatIf need no elevation when read access permits.' }
$operationMutex = [Threading.Mutex]::new($false, 'Global\ProxyGlassDev-InstallManager')
$lockTaken = $false
try {
    try { $lockTaken = $operationMutex.WaitOne(0) }
    catch [Threading.AbandonedMutexException] { $lockTaken = $true }
    if (-not $lockTaken) { throw 'Another development installation operation is running.' }
    # Recheck shared resources after confirmation, before touching any system state.
    Assert-Layout (Read-Layout) $marker; Assert-Service (Read-Service) $marker
    Assert-Ancestors $InstallDirectory; Assert-Ancestors $DataDirectory
    Assert-NoReparse $InstallDirectory -Tree
    if ($marker) { Assert-OwnedTree $InstallDirectory; Assert-DataRoot }
    if (-not $EvidenceDirectory) { $EvidenceDirectory = Join-Path $ProjectRoot 'build\dev-evidence' }
    $EvidenceDirectory = Full-Path $EvidenceDirectory
    Assert-Separate $EvidenceDirectory $InstallDirectory; Assert-Separate $EvidenceDirectory $DataDirectory
    Assert-NoReparse $EvidenceDirectory
    $evidence = Join-Path $EvidenceDirectory ((Get-Date -Format 'yyyyMMdd-HHmmss-ffff') + '-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $evidence | Out-Null
    [pscustomobject]@{ Plan = $summary; PreviousMarker = $marker; PreviousLayout = $layout; PreviousService = $service; Files = $files } | ConvertTo-Json -Depth 15 | Set-Content -LiteralPath (Join-Path $evidence 'before.json') -Encoding utf8NoBOM
    try {
        if ($Action -eq 'Install') {
            if (-not $marker) {
                New-Item -ItemType Directory -Path $InstallDirectory | Out-Null
                $acl = [Security.AccessControl.DirectorySecurity]::new()
                $acl.SetSecurityDescriptorSddlForm('O:BAG:BAD:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GRGX;;;AU)')
                Set-Acl -LiteralPath $InstallDirectory -AclObject $acl
                $marker = [pscustomobject]@{ Schema = 1; Product = $product; InstallationID = [guid]::NewGuid().ToString(); InstallDirectory = $InstallDirectory; DataDirectory = $DataDirectory; CertificateSHA256 = $certificateHash; State = 'Preparing'; Files = $files }
            } else { $marker.State = 'Preparing'; $marker.Files = $files }
            Write-Marker $marker
            foreach ($file in $files) {
                $destination = Join-Path $InstallDirectory $file.Path
                if (Test-Path -LiteralPath $destination) { throw "Refusing to overwrite remaining file: $destination" }
                New-Item -ItemType Directory -Path (Split-Path $destination -Parent) -Force | Out-Null
                Copy-Item -LiteralPath (Join-Path $Source $file.Path) -Destination $destination
            }
            Assert-Inventory $InstallDirectory $files; Assert-Signatures $InstallDirectory
        } else { $marker.State = 'Operating'; Write-Marker $marker }
        if ($Action -in 'Install', 'Repair') {
            Official-Command -Arguments @('service', 'install', '--working-directory', $DataDirectory)
            $installedService = Read-Service
            if (-not $installedService -or $installedService.State -ne 'Running' -or $installedService.StartMode -ne 'Auto') { throw 'Official install returned without a running automatic service.' }
            Assert-Service $installedService $marker
            Assert-DataRoot
            $base = [Microsoft.Win32.RegistryKey]::OpenBaseKey([Microsoft.Win32.RegistryHive]::LocalMachine, [Microsoft.Win32.RegistryView]::Registry64)
            try {
                $key = $base.CreateSubKey($registryPath)
                try {
                    $key.SetValue('LayoutVersion', 2, [Microsoft.Win32.RegistryValueKind]::DWord)
                    $key.SetValue('InstallationID', $marker.InstallationID)
                    $key.SetValue('DaemonDataDirectory', $DataDirectory)
                    $key.SetValue('ProxyGlassDevOwner', $product)
                    $key.SetValue('ProxyGlassDevInstallDirectory', $InstallDirectory)
                    $key.DeleteValue('ApplicationDataDirectory', $false)
                } finally { $key.Dispose() }
            } finally { $base.Dispose() }
            $marker.State = 'Installed'; Write-Marker $marker
        } else {
            Official-Command -Arguments @('service', 'uninstall')
            if (Read-Service) { throw 'Service still present after official uninstall; no files were removed.' }
            Assert-Inventory $InstallDirectory $files
            foreach ($file in $files) { Remove-Item -LiteralPath (Join-Path $InstallDirectory $file.Path) -Force }
            # Remove only now-empty inventory directories; never recurse or erase
            # unknown files, the marker, logs, daemon data, or per-user GUI data.
            $directories = @($files | ForEach-Object { Split-Path (Join-Path $InstallDirectory $_.Path) -Parent } | Sort-Object -Unique | Sort-Object Length -Descending)
            foreach ($directory in $directories) {
                while ($directory -ne $InstallDirectory -and (Is-Within $directory $InstallDirectory)) {
                    if ((Test-Path -LiteralPath $directory) -and @(Get-ChildItem -LiteralPath $directory -Force).Count -eq 0) { Remove-Item -LiteralPath $directory }
                    $directory = Split-Path $directory -Parent
                }
            }
            $marker.State = 'Uninstalled'; Write-Marker $marker
        }
        [pscustomobject]@{ Result = 'Success'; Action = $Action; InstallationID = $marker.InstallationID; DataRetained = $true } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $evidence 'result.json') -Encoding utf8NoBOM
        Write-Output "Completed $Action. Evidence: $evidence. Data retained: $DataDirectory"
    } catch {
        $_ | Out-String | Set-Content -LiteralPath (Join-Path $evidence 'failure.txt') -Encoding utf8NoBOM
        throw "Operation stopped. No automatic rollback was attempted; preserve $evidence and the installation marker for manual review. $($_.Exception.Message)"
    }
} finally {
    if ($lockTaken) { $operationMutex.ReleaseMutex() }
    $operationMutex.Dispose()
}
