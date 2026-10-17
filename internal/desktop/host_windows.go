package desktop

import (
	"os/exec"
	"syscall"
	"unsafe"
)

func init() {
	prepareCommand = func(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
	ReportError = func(message string) {
		text, _ := syscall.UTF16PtrFromString(message)
		title, _ := syscall.UTF16PtrFromString("GoToBedAlready")
		syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
	}
}
