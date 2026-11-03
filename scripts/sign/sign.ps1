# Sign only explicit application/installer EXEs. No timestamps or trust-store changes.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string[]]$Files,
    [string]$Pfx = 'build/cert/codesign.pfx',
    [string]$Certificate = 'build/cert/codesign.cer',
    [System.Security.SecureString]$Password,
    [string]$SignTool
)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/../env.ps1"
function Resolve-ProjectFile([string]$Path) {
    if (-not [IO.Path]::IsPathRooted($Path)) { $Path = Join-Path $ProjectRoot $Path }
    return (Resolve-Path -LiteralPath $Path).Path
}
$Pfx = Resolve-ProjectFile $Pfx
$Certificate = Resolve-ProjectFile $Certificate
$Files = @($Files | ForEach-Object { Resolve-ProjectFile $_ })
foreach ($file in $Files) { if ([IO.Path]::GetExtension($file) -ne '.exe') { throw 'Only explicit EXE files may be signed; do not re-sign upstream drivers or DLLs' } }
if (-not $SignTool) {
    $command = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($command) { $SignTool = $command.Source }
    else {
        $sdk = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits/10/bin'
        $candidates = @(Get-ChildItem -LiteralPath $sdk -Filter signtool.exe -Recurse -ErrorAction SilentlyContinue |
            Where-Object { $_.Directory.Name -eq 'x64' } | Sort-Object FullName -Descending)
        if ($candidates.Count -gt 0) { $SignTool = $candidates[0].FullName }
    }
}
if (-not $SignTool -or -not (Test-Path -LiteralPath $SignTool)) { throw 'Windows SDK signtool.exe was not found; no files were signed. Install Windows SDK or pass -SignTool.' }
if (-not $Password) {
    if ($env:CODESIGN_PFX_PASSWORD) { $Password = ConvertTo-SecureString $env:CODESIGN_PFX_PASSWORD -AsPlainText -Force }
    else { $Password = Read-Host 'Development PFX password' -AsSecureString }
}
& go run "$PSScriptRoot/check-cert.go" $Certificate
if ($LASTEXITCODE -ne 0) { throw 'Certificate specification check failed' }
$expected = New-Object Security.Cryptography.X509Certificates.X509Certificate2($Certificate)
$probe = $null
$imported = $null
try {
    # Ephemeral probe checks the PFX identity before any persistent store write.
    $probe = New-Object Security.Cryptography.X509Certificates.X509Certificate2($Pfx, $Password, ([Security.Cryptography.X509Certificates.X509KeyStorageFlags]::EphemeralKeySet))
    if ([Convert]::ToBase64String($probe.RawData) -cne [Convert]::ToBase64String($expected.RawData)) { throw 'PFX leaf certificate does not match expected certificate DER' }
    if (-not $probe.HasPrivateKey) { throw 'PFX has no private key' }
    $storePath = "Cert:\CurrentUser\My\$($expected.Thumbprint)"
    if (Test-Path -LiteralPath $storePath) { throw 'Signing certificate already exists in CurrentUser/My; refusing to replace or delete an existing key' }
    $imported = Import-PfxCertificate -FilePath $Pfx -CertStoreLocation 'Cert:\CurrentUser\My' -Password $Password
    foreach ($file in $Files) {
        & $SignTool sign /fd SHA256 /s My /sha1 $expected.Thumbprint $file
        if ($LASTEXITCODE -ne 0) { throw "signtool failed: $file" }
    }
    & go run "$PSScriptRoot/verify.go" -cert $Certificate @Files
    if ($LASTEXITCODE -ne 0) { throw 'Authenticode verification failed' }
} finally {
    if ($imported) { foreach ($cert in @($imported)) { Remove-Item -LiteralPath "Cert:\CurrentUser\My\$($cert.Thumbprint)" -DeleteKey -ErrorAction Stop } }
    if ($probe) { $probe.Dispose() }
    $expected.Dispose()
}
