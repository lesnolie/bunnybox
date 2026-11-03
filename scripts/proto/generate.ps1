param([ValidateSet('generate', 'check', 'inspect')][string]$Mode = 'generate')
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '../env.ps1')
Push-Location $ProjectRoot
try {
    $env:PATH = "$(Join-Path $env:GOPATH 'bin');$env:PATH"
    go run ./scripts/proto -mode $Mode
    if ($LASTEXITCODE -ne 0) { throw "Protocol $Mode failed ($LASTEXITCODE)" }
} finally { Pop-Location }
