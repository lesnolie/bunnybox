param(
    [string]$Executable = 'C:\Program Files\ProxyGlassDev\sing-box.exe',
    [string]$Out = 'build/validation-m0.json',
    [switch]$SmokeCore
)
. "$PSScriptRoot/env.ps1"
$Executable = (Resolve-Path -LiteralPath $Executable).Path
if ((Split-Path $Executable -Leaf) -ine 'sing-box.exe') { throw 'Expected the real GUI executable sing-box.exe' }
$OutputPath = [IO.Path]::GetFullPath((Join-Path $ProjectRoot $Out))
$BuildRoot = [IO.Path]::GetFullPath((Join-Path $ProjectRoot 'build')) + [IO.Path]::DirectorySeparatorChar
if (!$OutputPath.StartsWith($BuildRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'Evidence output must be inside build/' }
$Parent = Split-Path $OutputPath -Parent
New-Item -ItemType Directory -Path $Parent -Force | Out-Null
$Arguments = @('--diagnose')
if ($SmokeCore) { $Arguments = @('--smoke-core') }
$Raw = & $Executable @Arguments
$ProbeExit = $LASTEXITCODE
try { $Result = ($Raw -join "`n") | ConvertFrom-Json } catch { throw "Diagnostic did not return valid JSON (exit $ProbeExit)" }
$Daemon = Join-Path (Split-Path $Executable -Parent) 'resources/daemon/sing-box-daemon.exe'
$Record = [ordered]@{
    scope = 'local-m0-only'
    collected_at = (Get-Date).ToUniversalTime().ToString('o')
    os = [Environment]::OSVersion.VersionString
    architecture = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    session_id = (Get-Process -Id $PID).SessionId
    executable = $Executable
    gui_sha256 = (Get-FileHash -LiteralPath $Executable -Algorithm SHA256).Hash
    daemon_sha256 = (Get-FileHash -LiteralPath $Daemon -Algorithm SHA256).Hash
    operation = $Arguments[0]
    exit_code = $ProbeExit
    result = $Result
}
$Record | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $OutputPath -Encoding UTF8
Write-Output "Evidence saved: $OutputPath"
if ($ProbeExit -ne 0) { throw "M0 check did not pass (exit $ProbeExit); inspect saved evidence" }
