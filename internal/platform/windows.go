package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
)

var configureCommand = func(command *exec.Cmd) {}

type WindowsObserver struct {
	Script string
}

func systemExecutable(name string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", errors.New("наблюдение доступно только на Windows; используйте -demo")
	}
	root := os.Getenv("SystemRoot")
	if root == "" || !filepath.IsAbs(root) {
		return "", errors.New("SystemRoot недоступен")
	}
	return filepath.Join(root, "System32", name), nil
}

func (w WindowsObserver) execute(ctx context.Context, arguments ...string) ([]byte, error) {
	program, err := systemExecutable("WindowsPowerShell/v1.0/powershell.exe")
	if err != nil {
		return nil, err
	}
	args := append([]string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", w.Script}, arguments...)
	command := exec.CommandContext(ctx, program, args...)
	configureCommand(command)
	output, err := command.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Windows UI Automation не ответил: %w", ctx.Err())
		}
		return nil, fmt.Errorf("Windows UI Automation недоступен: %w", err)
	}
	return []byte(strings.TrimPrefix(string(output), "\ufeff")), nil
}

func (w WindowsObserver) Processes(ctx context.Context) ([]domain.Process, error) {
	output, err := w.execute(ctx, "-Mode", "list")
	if err != nil {
		return nil, err
	}
	processes := []domain.Process{}
	if err := json.Unmarshal(output, &processes); err != nil {
		return nil, fmt.Errorf("decode processes: %w", err)
	}
	return processes, nil
}

func (w WindowsObserver) Read(ctx context.Context, process domain.Process) (domain.Sample, error) {
	output, err := w.execute(ctx, "-Mode", "read", "-TargetId", strconv.Itoa(process.PID), "-StartedAt", process.StartedAt)
	if err != nil {
		return domain.Sample{}, err
	}
	var sample domain.Sample
	if err := json.Unmarshal(output, &sample); err != nil {
		return domain.Sample{}, fmt.Errorf("decode progress: %w", err)
	}
	return sample, nil
}

type WindowsPower struct{}

func (WindowsPower) Shutdown(ctx context.Context) error {
	program, err := systemExecutable("shutdown.exe")
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, program, "/s", "/t", "0")
	configureCommand(command)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("shutdown: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func OpenBrowser(url string) error {
	program, err := systemExecutable("rundll32.exe")
	if err != nil {
		return err
	}
	command := exec.Command(program, "url.dll,FileProtocolHandler", url)
	configureCommand(command)
	if err := command.Start(); err != nil {
		return err
	}
	go command.Wait()
	return nil
}
