param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
. "$PSScriptRoot/env.ps1"
$ManifestPath = Join-Path $ProjectRoot 'third_party/mygo/plugins/sqlite/mygo-plugin.json'
$Manifest = Get-Content -LiteralPath $ManifestPath -Raw | ConvertFrom-Json
$Library = $Manifest.libraries | Where-Object name -eq 'mygo-sqlite3'
$Asset = $Library.files."windows-$Arch"
if (!$Asset -or $Asset.name -ne 'mygo-sqlite3.dll' -or $Asset.sha256 -notmatch '^[a-f0-9]{64}$' -or !$Asset.url.StartsWith('https://github.com/egoist/mygo/releases/download/')) { throw 'Invalid locked SQLite runtime manifest' }
$Destination = Join-Path $ProjectRoot 'build/app'
if ($Arch -ne 'amd64') { $Destination = Join-Path $ProjectRoot "build/$Arch/app" }
New-Item -ItemType Directory -Path $Destination -Force | Out-Null
$Path = Join-Path $Destination $Asset.name
if (!(Test-Path -LiteralPath $Path) -or (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Asset.sha256) {
    $Temp = Join-Path $Destination ([guid]::NewGuid().ToString('N') + '.part')
    try {
        Invoke-WebRequest -Uri $Asset.url -OutFile $Temp -TimeoutSec 120
        if ((Get-FileHash -LiteralPath $Temp -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Asset.sha256) { throw 'SQLite runtime digest mismatch' }
        Move-Item -LiteralPath $Temp -Destination $Path -Force
    } finally { if (Test-Path -LiteralPath $Temp) { Remove-Item -LiteralPath $Temp } }
}
Write-Output "Staged locked SQLite runtime: $Path"
