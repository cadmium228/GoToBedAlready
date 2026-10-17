package platform

import (
	"os/exec"
	"syscall"
)

func init() {
	configureCommand = func(command *exec.Cmd) {
		command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
}
