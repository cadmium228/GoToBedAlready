package platform

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPowerShellSyntax(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell required")
	}
	program, err := systemExecutable("WindowsPowerShell/v1.0/powershell.exe")
	if err != nil {
		t.Fatal(err)
	}
	path, err := filepath.Abs("../../scripts/observe.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, "-NoProfile", "-NonInteractive", "-Command", "$tokens=$null; $errors=$null; [System.Management.Automation.Language.Parser]::ParseFile($env:GTB_SCRIPT,[ref]$tokens,[ref]$errors) | Out-Null; if ($errors.Count) { $errors | Out-String | Write-Error; exit 1 }")
	command.Env = append(command.Environ(), "GTB_SCRIPT="+path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell syntax: %v\n%s", err, output)
	}
}

func TestWindowsProcessListing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows required")
	}
	path, err := filepath.Abs("../../scripts/observe.ps1")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := (Windows{Script: path}).Processes(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDemoPowerIsAlwaysBlocked(t *testing.T) {
	if (&Demo{}).Shutdown(context.Background()) == nil {
		t.Fatal("demo shutdown allowed")
	}
}
