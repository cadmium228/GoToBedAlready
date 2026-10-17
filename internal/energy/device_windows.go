package energy

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

type windowsDevice struct {
	execution *syscall.LazyProc
	lastInput *syscall.LazyProc
	ticks     *syscall.LazyProc
	message   *syscall.LazyProc
}

func init() {
	NewDevice = func() Device {
		kernel := syscall.NewLazyDLL("kernel32.dll")
		user := syscall.NewLazyDLL("user32.dll")
		return &windowsDevice{execution: kernel.NewProc("SetThreadExecutionState"), lastInput: user.NewProc("GetLastInputInfo"), ticks: kernel.NewProc("GetTickCount64"), message: user.NewProc("SendMessageTimeoutW")}
	}
}

func (w *windowsDevice) Input() (Input, error) {
	info := struct {
		Size uint32
		Tick uint32
	}{Size: 8}
	result, _, err := w.lastInput.Call(uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		return Input{}, fmt.Errorf("GetLastInputInfo: %v", err)
	}
	now, _, _ := w.ticks.Call()
	return Input{Sequence: info.Tick, Idle: time.Duration(uint32(now)-info.Tick) * time.Millisecond}, nil
}

func (w *windowsDevice) Request(system, display bool) error {
	flags := uintptr(0x80000000)
	if system {
		flags |= 0x1
	}
	if display {
		flags |= 0x2
	}
	result, _, err := w.execution.Call(flags)
	if result == 0 {
		return fmt.Errorf("SetThreadExecutionState: %v", err)
	}
	return nil
}

func (w *windowsDevice) Display(on bool) error {
	parameter := uintptr(2)
	if on {
		parameter = ^uintptr(0)
	}
	var response uintptr
	result, _, err := w.message.Call(0xffff, 0x0112, 0xf170, parameter, 0x2, 500, uintptr(unsafe.Pointer(&response)))
	if result == 0 {
		return fmt.Errorf("SC_MONITORPOWER: %v", err)
	}
	return nil
}
