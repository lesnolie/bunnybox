# Development certificate only. Never import into Trusted Root Certification Authorities.
[CmdletBinding()]
param(
    [string]$Subject = 'CN=sing-box GUI Development Code Signing, O=SingBoxWin',
    [ValidateRange(2, 10)][int]$Years = 10,
    [string]$Out = 'build/cert',
    [System.Security.SecureString]$Password
)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/../env.ps1"
if (-not $Password) {
    if ($env:CODESIGN_PFX_PASSWORD) { $Password = ConvertTo-SecureString $env:CODESIGN_PFX_PASSWORD -AsPlainText -Force }
    else { $Password = Read-Host 'Development PFX password' -AsSecureString }
}
if ($Password.Length -eq 0) { throw 'PFX password must not be empty' }
if (-not [IO.Path]::IsPathRooted($Out)) { $Out = Join-Path $ProjectRoot $Out }
$Out = [IO.Path]::GetFullPath($Out)
$buildRoot = [IO.Path]::GetFullPath((Join-Path $ProjectRoot 'build')) + [IO.Path]::DirectorySeparatorChar
if (-not $Out.StartsWith($buildRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'Development private keys must be written below the project build directory' }
New-Item -ItemType Directory -Force $Out | Out-Null
$pfxPath = Join-Path $Out 'codesign.pfx'
$cerPath = Join-Path $Out 'codesign.cer'
if ((Test-Path $pfxPath) -or (Test-Path $cerPath)) { throw 'Certificate already exists; reuse it or choose a new output directory' }
$cert = $null
try {
    $notBefore = (Get-Date).ToUniversalTime().AddDays(-1)
    $serial = New-Object byte[] 16
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($serial) } finally { $rng.Dispose() }
    # Prefix a sign byte: CertEnroll otherwise emits a negative ASN.1 INTEGER
    # when the random serial's highest bit is set. Preserve all 128 random bits.
    $cert = New-SelfSignedCertificate -Type CodeSigningCert -Subject $Subject `
        -KeyAlgorithm RSA -KeyLength 3072 -HashAlgorithm SHA256 -KeyExportPolicy Exportable `
        -KeyUsage DigitalSignature -SerialNumber ('00' + [BitConverter]::ToString($serial).Replace('-', '')) `
        -NotBefore $notBefore -NotAfter $notBefore.AddYears($Years) -CertStoreLocation 'Cert:\CurrentUser\My'
    Export-PfxCertificate -Cert $cert -FilePath $pfxPath -Password $Password | Out-Null
    Export-Certificate -Cert $cert -FilePath $cerPath -Type CERT | Out-Null
    & go run "$PSScriptRoot/check-cert.go" $cerPath
    if ($LASTEXITCODE -ne 0) { throw 'Certificate specification check failed' }
    $hash = (Get-FileHash -Algorithm SHA256 $cerPath).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $Out 'codesign.cer.sha256'), $hash + "`n")
    Write-Host "Development certificate exported to $Out"
} finally {
    if ($cert) { Remove-Item -LiteralPath "Cert:\CurrentUser\My\$($cert.Thumbprint)" -DeleteKey -ErrorAction Stop }
}
