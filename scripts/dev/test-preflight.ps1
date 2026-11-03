#requires -Version 7.2
# Low-risk checks only: load function definitions, never the management entrypoint.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$tokens = $null; $errors = $null
$script = Join-Path $PSScriptRoot 'manage.ps1'
$ast = [Management.Automation.Language.Parser]::ParseFile($script, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
foreach ($function in $ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] }, $false)) {
    Invoke-Expression $function.Extent.Text
}
$checks = 0
$TemplateACL = [Security.AccessControl.RawSecurityDescriptor]::new('O:BAG:BAD:(A;CI;KA;;;CO)(A;CI;KA;;;BU)')
if (!(Is-RegistryCreatorTemplate $TemplateACL.DiscretionaryAcl[0])) { throw 'CREATOR OWNER template must not be mistaken for a user group.' }
if (Is-RegistryCreatorTemplate $TemplateACL.DiscretionaryAcl[1]) { throw 'Actual BUILTIN Users write access must not be exempted.' }
$checks += 2
function Must-Reject([scriptblock]$Body, [string]$Label) {
    $rejected = $false
    try { & $Body | Out-Null } catch { $rejected = $true }
    if (-not $rejected) { throw "Expected rejection: $Label" }
    $script:checks++
}
$InstallDirectory = 'C:\Program Files\ProxyGlassDev'
$DataDirectory = 'C:\ProgramData\ProxyGlassDev'
$markerName = '.proxyglass-dev-install.json'
$product = 'ProxyGlassDev/7ac8d978-3e72-4bdb-8bf8-d8b10965e2b1'
$marker = [pscustomobject]@{ InstallationID = [guid]::NewGuid().ToString() }
Must-Reject { Full-Path 'C:\Program Files\..\Windows' } 'dot-segment destination'
Must-Reject { Full-Path '\\server\share\ProxyGlassDev' } 'network destination'
Must-Reject { Full-Path 'C:\' } 'volume root destination'
Must-Reject { Full-Path 'C:\safe\file:stream' } 'alternate data stream'
Must-Reject { Assert-Separate $InstallDirectory ($InstallDirectory + '\data') } 'nested data directory'
Assert-Separate $InstallDirectory $DataDirectory
$checks++
$service = [pscustomobject]@{ PathName = '"C:\Program Files\ProxyGlassDev\resources\daemon\sing-box-daemon.exe" run --working-directory C:\ProgramData\ProxyGlassDev'; StartName = 'LocalSystem' }
Assert-Service $service $marker
$checks++
Must-Reject { Assert-Service $service $null } 'unowned official service'
$service.PathName += ' --allow-unsafe-installation-directory-permissions'
Must-Reject { Assert-Service $service $marker } 'unsafe or extra service arguments'
$service.PathName = '"C:\Program Files\sing-box\resources\daemon\sing-box-daemon.exe" run --working-directory C:\ProgramData\ProxyGlassDev'
Must-Reject { Assert-Service $service $marker } 'third-party service executable'
Must-Reject { Assert-Layout ([ordered]@{ LayoutVersion = 2; InstallationID = 'official-client' }) $marker } 'foreign registry layout'
Must-Reject { Assert-Layout ([ordered]@{ LayoutVersion = 2 }) $null } 'unowned registry layout'
$layout = [ordered]@{ LayoutVersion = 2; InstallationID = $marker.InstallationID; ProxyGlassDevOwner = $product; ProxyGlassDevInstallDirectory = $InstallDirectory; DaemonDataDirectory = $DataDirectory }
Assert-Layout $layout $marker
$checks++
$layout.ApplicationDataDirectory = 'C:\OtherData'
Must-Reject { Assert-Layout $layout $marker } 'unexpected shared GUI data'

# A concrete preflight invokes only read checks. This machine has a dangerous
# C:\ root ACE; assert it is rejected before service installation is possible.
$trusted = @('S-1-5-18', 'S-1-5-32-544', 'S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464')
$raw = [Security.AccessControl.RawSecurityDescriptor]::new((Get-Acl C:\).GetSecurityDescriptorBinaryForm(), 0)
$unsafeRoot = @($raw.DiscretionaryAcl | Where-Object {
    $_ -is [Security.AccessControl.CommonAce] -and
    -not ($_.AceFlags -band [Security.AccessControl.AceFlags]::InheritOnly) -and
    $_.AceQualifier -eq [Security.AccessControl.AceQualifier]::AccessAllowed -and
    ($_.AccessMask -band 0x500D0040) -ne 0 -and $trusted -notcontains $_.SecurityIdentifier.Value
}).Count -gt 0
if ($unsafeRoot) { Must-Reject { Assert-Ancestors $InstallDirectory } 'unsafe actual C drive root'; Write-Output 'Actual machine: C:\ ancestor permissions block safe installation, as required.' }
Write-Output "Preflight checks passed: $checks. No service, registry, trust-store, ACL, or installation mutations were performed."
