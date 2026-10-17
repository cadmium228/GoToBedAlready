package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
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
	if _, err := (WindowsObserver{Script: path}).Processes(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestDemoPowerIsAlwaysBlocked(t *testing.T) {
	if (DisabledPower{}).Shutdown(context.Background()) == nil {
		t.Fatal("demo shutdown allowed")
	}
}

func TestWindowsProgressObservation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("interactive Windows UI Automation required")
	}
	program, err := systemExecutable("WindowsPowerShell/v1.0/powershell.exe")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	valuePath := filepath.Join(directory, "value.txt")
	fixturePath := filepath.Join(directory, "fixture.ps1")
	fixtureScript := `Add-Type -AssemblyName System.Windows.Forms
[Windows.Forms.Application]::EnableVisualStyles()
$window = New-Object Windows.Forms.Form
$window.Text = 'GoToBedAlready UI Automation Test'
$bar = New-Object Windows.Forms.ProgressBar
$bar.Width = 250
$bar.Value = 25
$window.Controls.Add($bar)
$timer = New-Object Windows.Forms.Timer
$timer.Interval = 200
$timer.Add_Tick({
    try { $bar.Value = [int](Get-Content -LiteralPath $env:GTB_TEST_VALUE -Raw) } catch {}
})
$timer.Start()
$window.ShowDialog() | Out-Null
`
	if err := os.WriteFile(valuePath, []byte("25"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixturePath, []byte(fixtureScript), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", fixturePath)
	command.Env = append(command.Environ(), "GTB_TEST_VALUE="+valuePath)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		command.Process.Kill()
		command.Wait()
	}()
	observerPath, err := filepath.Abs("../../scripts/observe.ps1")
	if err != nil {
		t.Fatal(err)
	}
	observer := WindowsObserver{Script: observerPath}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var target domain.Process
	for target.PID == 0 {
		select {
		case <-ctx.Done():
			t.Fatal("fixture window did not become available")
		case <-ticker.C:
		}
		processes, err := observer.Processes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, process := range processes {
			if process.PID == command.Process.Pid && process.Title == "GoToBedAlready UI Automation Test" {
				target = process
			}
		}
	}
	first, err := observer.Read(ctx, target)
	if err != nil || !first.Valid() || first.Percent != 25 {
		diagnostic := exec.CommandContext(ctx, program, "-NoProfile", "-NonInteractive", "-Command", "$p=Get-Process -Id $env:GTB_TARGET; Add-Type -AssemblyName UIAutomationClient; Add-Type -AssemblyName UIAutomationTypes; $root=[Windows.Automation.AutomationElement]::FromHandle($p.MainWindowHandle); $root.FindAll([Windows.Automation.TreeScope]::Subtree,[Windows.Automation.Condition]::TrueCondition) | ForEach-Object { @{name=$_.Current.Name; class=$_.Current.ClassName; type=$_.Current.ControlType.ProgrammaticName; enabled=$_.Current.IsEnabled; patterns=@($_.GetSupportedPatterns() | ForEach-Object {$_.ProgrammaticName})} } | ConvertTo-Json -Depth 5 -Compress")
		diagnostic.Env = append(diagnostic.Environ(), "GTB_TARGET="+strconv.Itoa(target.PID))
		output, _ := diagnostic.CombinedOutput()
		t.Fatalf("expected 25%% progress: %+v, %v; target=%+v; tree=%s", first, err, target, output)
	}
	if err := os.WriteFile(valuePath, []byte("100"), 0600); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case <-ctx.Done():
			t.Fatal("100 percent was not observed")
		case <-ticker.C:
		}
		last, err := observer.Read(ctx, target)
		if err != nil || !last.Valid() || last.Source != first.Source {
			t.Fatalf("unstable progress source: %+v, %v", last, err)
		}
		if last.Percent == 100 {
			break
		}
	}
	target.StartedAt = "different-process-identity"
	stale, err := observer.Read(ctx, target)
	if err != nil || stale.Available {
		t.Fatalf("reused PID must be unavailable: %+v, %v", stale, err)
	}
}
