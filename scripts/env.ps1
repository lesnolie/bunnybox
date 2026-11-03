$ErrorActionPreference = 'Stop'
$ProjectRoot = Split-Path $PSScriptRoot -Parent
$env:GOPATH = Join-Path $ProjectRoot '.cache/go'
$env:GOCACHE = Join-Path $ProjectRoot '.cache/build'
$env:GOTELEMETRYDIR = Join-Path $ProjectRoot '.cache/telemetry'
$env:GOTOOLCHAIN = 'auto'
