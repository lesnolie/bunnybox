. "$PSScriptRoot/version.ps1"
& "$PSScriptRoot/check-upstream.ps1"
& "$PSScriptRoot/stage-sqlite.ps1"
Push-Location $ProjectRoot
try {
    # Icon, manifest and version information of the executable.
    & go run ./scripts/winres -icon assets/icon-256.png -version $ProductVersion -o rsrc_windows_amd64.syso
    if ($LASTEXITCODE -ne 0) { throw 'resource build failed' }
    & go build -trimpath -o build/app/sing-box.exe .
    if ($LASTEXITCODE -ne 0) { throw 'GUI build failed' }
} finally { Pop-Location }
