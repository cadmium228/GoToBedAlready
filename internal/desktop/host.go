package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

var prepareCommand = func(command *exec.Cmd) {}
var ReportError = func(message string) {}

func Run(ctx context.Context, assets, url string) error {
	if runtime.GOOS != "windows" {
		return errors.New("WebView2 доступен только на Windows")
	}
	executable := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	command := exec.CommandContext(ctx, executable, "-NoLogo", "-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(assets, "scripts", "desktop.ps1"))
	command.Env = append(command.Environ(), "GTB_URL="+url, "GTB_ASSETS="+assets, "GTB_PARENT="+strconv.Itoa(os.Getpid()))
	prepareCommand(command)
	if output, err := command.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return errors.New("Не удалось открыть WebView2. Установите Microsoft Edge WebView2 Runtime и проверьте папку runtime/webview2. " + string(output))
	}
	return nil
}
