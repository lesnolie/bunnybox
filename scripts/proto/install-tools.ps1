$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '../env.ps1')
$manifest = Get-Content (Join-Path $PSScriptRoot 'manifest.json') -Raw | ConvertFrom-Json
$goVersion = ($manifest.protocGenGo -split ' ')[-1]
$grpcVersion = 'v' + ($manifest.protocGenGoGrpc -split ' ')[-1]
go install "google.golang.org/protobuf/cmd/protoc-gen-go@$goVersion"
if ($LASTEXITCODE -ne 0) { throw 'Installing protoc-gen-go failed' }
go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@$grpcVersion"
if ($LASTEXITCODE -ne 0) { throw 'Installing protoc-gen-go-grpc failed' }
Write-Output "Go plugins installed in $env:GOPATH/bin; protoc must report: $($manifest.protoc)"
