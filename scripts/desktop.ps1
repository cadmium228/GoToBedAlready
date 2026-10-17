$ErrorActionPreference = 'Stop'
$root = Join-Path $env:GTB_ASSETS 'runtime\webview2'
$architecture = switch ([Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture.ToString()) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw 'A 64-bit PowerShell host is required' }
}
$env:PATH = (Join-Path $root $architecture) + ';' + $env:PATH
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
Add-Type -Path (Join-Path $root 'Microsoft.Web.WebView2.Core.dll')
Add-Type -Path (Join-Path $root 'Microsoft.Web.WebView2.WinForms.dll')
$references = @(
    'System.Windows.Forms.dll',
    'System.Drawing.dll',
    'System.dll',
    (Join-Path $root 'Microsoft.Web.WebView2.Core.dll'),
    (Join-Path $root 'Microsoft.Web.WebView2.WinForms.dll')
)
Add-Type -Path (Join-Path $env:GTB_ASSETS 'scripts\DesktopHost.cs') -ReferencedAssemblies $references
[GoToBedAlready.DesktopHost]::Run($env:GTB_URL)
