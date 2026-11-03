param([ValidateSet('amd64','arm64')][string]$Arch = 'amd64')
. "$PSScriptRoot/../env.ps1"
& "$PSScriptRoot/../check-upstream.ps1"
$Output = Join-Path $ProjectRoot "build/$Arch/resources/daemon/sing-box-daemon.exe"
Push-Location (Join-Path $ProjectRoot 'third_party/sing-box')
try {
    & go run ./cmd/internal/build_boxdd "-target=windows/$Arch" "-output=$Output"
    if ($LASTEXITCODE -ne 0) { throw 'Official daemon build failed' }
} finally { Pop-Location }
& "$PSScriptRoot/stage-runtime.ps1" -Arch $Arch
& "$PSScriptRoot/../check-upstream.ps1"
