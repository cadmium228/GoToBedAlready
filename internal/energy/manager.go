package energy

import (
	"context"
	"runtime"
	"sync"
	"time"
)

type Device interface {
	Input() (Input, error)
	Request(system, display bool) error
	Display(on bool) error
}

type StateSource interface {
	EnergyState() (Options, bool)
}

type Status struct {
	Supported bool    `json:"supported"`
	Applied   Options `json:"applied"`
	Warning   string  `json:"warning"`
}

type Manager struct {
	mu     sync.Mutex
	device Device
	source StateSource
	status Status
}

func New(device Device, source StateSource) *Manager {
	return &Manager{device: device, source: source, status: Status{Supported: device != nil}}
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *Manager) Run(ctx context.Context) {
	if m.device == nil {
		<-ctx.Done()
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var policy Policy
	var displayError error
	displayMayBeOff := false
	defer func() {
		if displayMayBeOff {
			m.device.Display(true)
		}
		m.device.Request(false, false)
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		options, monitoring := m.source.EnergyState()
		input, err := m.device.Input()
		if err == nil {
			candidate := policy
			decision := candidate.Step(options, monitoring, input)
			err = m.device.Request(decision.SystemRequired, decision.DisplayRequired)
			if err == nil && decision.DisplayChange != 0 {
				displayMayBeOff = displayMayBeOff || decision.DisplayChange < 0
				displayError = m.device.Display(decision.DisplayChange > 0)
				if displayError == nil && decision.DisplayChange > 0 {
					displayMayBeOff = false
				}
			}
			if err == nil {
				policy = candidate
			}
		}
		if err == nil {
			err = displayError
		}
		m.mu.Lock()
		if err != nil {
			m.status.Warning = "Не удалось применить параметры питания: " + err.Error()
		} else {
			m.status.Applied = options
			m.status.Warning = ""
		}
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
