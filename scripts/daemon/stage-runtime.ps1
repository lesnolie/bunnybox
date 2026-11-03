param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
. "$PSScriptRoot/../env.ps1"
$Destination = Join-Path $ProjectRoot "build/$Arch/resources/daemon"
if (!(Test-Path (Join-Path $Destination 'sing-box-daemon.exe'))) { throw 'Build the daemon first' }
# Both supported Windows architectures use with_naive_outbound/with_purego.
& {
    Push-Location (Join-Path $ProjectRoot 'third_party/sing-box')
    try {
        $ModuleOutput = & go list -m -json "github.com/sagernet/cronet-go/lib/windows_$Arch"
        if ($LASTEXITCODE -ne 0) { throw 'Cannot resolve locked cronet runtime' }
        $Module = $ModuleOutput | ConvertFrom-Json
        if (!$Module.Dir) {
            $Downloaded = & go mod download -json "$($Module.Path)@$($Module.Version)"
            if ($LASTEXITCODE -ne 0) { throw 'Cannot download locked cronet runtime' }
            $Module = $Downloaded | ConvertFrom-Json
        }
        if (!$Module.Dir -or !$Module.Sum) { throw 'Missing cronet module provenance' }
        $Library = Join-Path $Module.Dir 'libcronet.dll'
        if (!(Test-Path -LiteralPath $Library)) { throw 'Locked module does not contain libcronet.dll' }
        Copy-Item -LiteralPath $Library -Destination $Destination -Force
        $OriginalHash = (Get-FileHash -LiteralPath $Library -Algorithm SHA256).Hash
        $StagedHash = (Get-FileHash -LiteralPath (Join-Path $Destination 'libcronet.dll') -Algorithm SHA256).Hash
        if ($OriginalHash -ne $StagedHash) { throw 'Cronet staging hash mismatch' }
        [ordered]@{ module = $Module.Path; version = $Module.Version; sum = $Module.Sum; sha256 = $StagedHash } |
            ConvertTo-Json | Set-Content (Join-Path $Destination 'cronet-source.json') -Encoding UTF8
        Write-Output "Staged libcronet.dll ($StagedHash)"
    } finally { Pop-Location }
}
