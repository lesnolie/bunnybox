# Drives the installed (normal, non-e2e) GUI through Windows UI Automation for
# live-service checks. Launches only its own window with an isolated data
# directory and closes only that PID. Network-changing controls are refused.
param(
    [ValidateSet('Launch', 'Tree', 'Click', 'Type', 'Shot', 'Close')][string]$Action = 'Tree',
    [string]$Name,
    [string]$Text,
    [string]$Out,
    [switch]$AllowNetwork, # only for an agreed real-proxy test window
    [string]$Run = (Join-Path $PSScriptRoot '../../build/ui-live')
)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes, System.Windows.Forms, System.Drawing
$Run = [IO.Path]::GetFullPath($Run)
$PidFile = Join-Path $Run 'pid.txt'
# The HTTP proxy switch and mode buttons change host networking; this driver
# never clicks them. Start/Stop only run a core for the selected profile.
$Forbidden = @('HTTP Proxy', '系统 HTTP 代理', 'Rule', 'Global', 'Direct')

function Window {
    $id = [int](Get-Content $PidFile)
    $cond = [Windows.Automation.PropertyCondition]::new([Windows.Automation.AutomationElement]::ProcessIdProperty, $id)
    for ($i = 0; $i -lt 50; $i++) {
        $w = [Windows.Automation.AutomationElement]::RootElement.FindFirst([Windows.Automation.TreeScope]::Children, $cond)
        if ($w) { return $w }
        Start-Sleep -Milliseconds 200
    }
    throw "window for PID $id not found"
}
# Find returns the first element named label that supports one of patterns
# (any element without patterns): a form's text label shares its field's name.
function Find([string]$label, [Windows.Automation.AutomationPattern[]]$patterns = @()) {
    $cond = [Windows.Automation.PropertyCondition]::new([Windows.Automation.AutomationElement]::NameProperty, $label)
    for ($i = 0; $i -lt 25; $i++) {
        foreach ($e in (Window).FindAll([Windows.Automation.TreeScope]::Descendants, $cond)) {
            if ($patterns.Count -eq 0) { return $e }
            foreach ($p in $patterns) { $x = $null; if ($e.TryGetCurrentPattern($p, [ref]$x)) { return $e } }
        }
        Start-Sleep -Milliseconds 200
    }
    throw "element '$label' not found"
}

switch ($Action) {
    'Launch' {
        New-Item -ItemType Directory -Force (Join-Path $Run 'data') | Out-Null
        $p = Start-Process 'P:\ProxyGlassDev\sing-box.exe' -ArgumentList '--data-dir', (Join-Path $Run 'data') -PassThru
        Set-Content $PidFile $p.Id
        $null = Window
        "launched PID $($p.Id)"
    }
    'Tree' {
        $walker = [Windows.Automation.TreeWalker]::ControlViewWalker
        function Dump($e, $depth) {
            $c = $e.Current
            $r = $c.BoundingRectangle
            '{0}{1} "{2}" [{3:n0},{4:n0} {5:n0}x{6:n0}]' -f ('  ' * $depth), $c.ControlType.ProgrammaticName.Replace('ControlType.', ''), $c.Name, $r.X, $r.Y, $r.Width, $r.Height
            $child = $walker.GetFirstChild($e)
            while ($child) { Dump $child ($depth + 1); $child = $walker.GetNextSibling($child) }
        }
        Dump (Window) 0
    }
    'Click' {
        if (!$AllowNetwork -and $Forbidden -contains $Name) { throw "refusing network control '$Name'" }
        $e = Find $Name @([Windows.Automation.InvokePattern]::Pattern, [Windows.Automation.ExpandCollapsePattern]::Pattern, [Windows.Automation.TogglePattern]::Pattern, [Windows.Automation.SelectionItemPattern]::Pattern)
        $pattern = $null
        if ($e.TryGetCurrentPattern([Windows.Automation.InvokePattern]::Pattern, [ref]$pattern)) { $pattern.Invoke() }
        elseif ($e.TryGetCurrentPattern([Windows.Automation.ExpandCollapsePattern]::Pattern, [ref]$pattern)) { $pattern.Expand() }
        elseif ($e.TryGetCurrentPattern([Windows.Automation.TogglePattern]::Pattern, [ref]$pattern)) { $pattern.Toggle() }
        elseif ($e.TryGetCurrentPattern([Windows.Automation.SelectionItemPattern]::Pattern, [ref]$pattern)) { $pattern.Select() }
        else { throw "'$Name' has no invoke pattern" }
        "clicked $Name"
    }
    'Type' {
        $e = Find $Name @([Windows.Automation.ValuePattern]::Pattern)
        $pattern = $null
        if (!$e.TryGetCurrentPattern([Windows.Automation.ValuePattern]::Pattern, [ref]$pattern)) { throw "'$Name' has no value pattern" }
        # Native argument passing drops quotes; JSON must come from a file.
        if ($Text.StartsWith('@')) { $Text = [IO.File]::ReadAllText([IO.Path]::GetFullPath($Text.Substring(1))) }
        $pattern.SetValue($Text)
        "typed into $Name"
    }
    'Shot' {
        $r = (Window).Current.BoundingRectangle
        $bmp = [Drawing.Bitmap]::new([int]$r.Width, [int]$r.Height)
        $g = [Drawing.Graphics]::FromImage($bmp)
        $g.CopyFromScreen([int]$r.X, [int]$r.Y, 0, 0, $bmp.Size)
        $bmp.Save($Out, [Drawing.Imaging.ImageFormat]::Png)
        $g.Dispose(); $bmp.Dispose()
        "saved $Out"
    }
    'Close' {
        $id = [int](Get-Content $PidFile)
        $w = Window
        $w.GetCurrentPattern([Windows.Automation.WindowPattern]::Pattern).Close()
        try { Wait-Process -Id $id -Timeout 10 -ErrorAction Stop } catch {} # already exited is fine
        if (Get-Process -Id $id -ErrorAction SilentlyContinue) { throw "PID $id still running" }
        "closed PID $id"
    }
}
