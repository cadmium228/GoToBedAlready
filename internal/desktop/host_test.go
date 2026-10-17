package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestManagedHostCompilesOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows .NET Framework required")
	}
	assets, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(assets, "runtime", "webview2", "Microsoft.Web.WebView2.Core.dll")); err != nil {
		t.Fatal("run python scripts/fetch_webview.py first: ", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	source := `$ErrorActionPreference='Stop'; $root=Join-Path $env:GTB_ASSETS 'runtime\webview2'; Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; Add-Type -Path (Join-Path $root 'Microsoft.Web.WebView2.Core.dll'); Add-Type -Path (Join-Path $root 'Microsoft.Web.WebView2.WinForms.dll'); Add-Type -Path (Join-Path $env:GTB_ASSETS 'scripts\DesktopHost.cs') -ReferencedAssemblies @('System.Windows.Forms.dll','System.Drawing.dll','System.dll',(Join-Path $root 'Microsoft.Web.WebView2.Core.dll'),(Join-Path $root 'Microsoft.Web.WebView2.WinForms.dll')); Add-Type -Path (Join-Path $env:GTB_ASSETS 'scripts\native-progress.cs')`
	executable := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	command := exec.CommandContext(ctx, executable, "-NoProfile", "-NonInteractive", "-STA", "-Command", source)
	command.Env = append(command.Environ(), "GTB_ASSETS="+assets)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("host compilation: %v\n%s", err, output)
	}
}
