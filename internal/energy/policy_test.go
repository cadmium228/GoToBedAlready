package energy

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestScreenSleepsImmediatelyThenWaitsAfterInput(t *testing.T) {
	var policy Policy
	options := Options{BlankDisplay: true, PreventSleep: true}
	first := policy.Step(options, true, Input{Sequence: 1})
	if first.DisplayChange != -1 || first.DisplayRequired || !first.SystemRequired {
		t.Fatalf("first: %+v", first)
	}
	unchanged := policy.Step(options, true, Input{Sequence: 1, Idle: time.Second})
	if unchanged.DisplayChange != 0 {
		t.Fatal("repeated screen-off command")
	}
	wake := policy.Step(options, true, Input{Sequence: 2})
	if wake.DisplayChange != 0 || !wake.DisplayRequired {
		t.Fatalf("input: %+v", wake)
	}
	for _, idle := range []time.Duration{time.Second, 9 * time.Second} {
		if decision := policy.Step(options, true, Input{Sequence: 2, Idle: idle}); decision.DisplayChange != 0 {
			t.Fatal("blanked too early")
		}
	}
	if decision := policy.Step(options, true, Input{Sequence: 2, Idle: 10 * time.Second}); decision.DisplayChange != -1 {
		t.Fatal("not blanked at ten seconds")
	}
}

func TestCancellationAndSwitchOffRestoreDisplay(t *testing.T) {
	for _, monitoring := range []bool{true, false} {
		var policy Policy
		policy.Step(Options{BlankDisplay: true, PreventSleep: true}, true, Input{Sequence: 1})
		decision := policy.Step(Options{PreventSleep: true}, monitoring, Input{Sequence: 1})
		if decision.DisplayChange != 1 || !decision.SystemRequired || !decision.DisplayRequired {
			t.Fatalf("restore: %+v", decision)
		}
		if next := policy.Step(Options{PreventSleep: true}, false, Input{}); next.DisplayChange != 0 {
			t.Fatal("repeated wake command")
		}
	}
}

func TestPreventSleepDoesNotDependOnMonitoring(t *testing.T) {
	var policy Policy
	for _, active := range []bool{false, true} {
		decision := policy.Step(Options{PreventSleep: true}, active, Input{})
		if !decision.SystemRequired || !decision.DisplayRequired || decision.DisplayChange != 0 {
			t.Fatal(decision)
		}
	}
	decision := policy.Step(Options{}, false, Input{})
	if decision.SystemRequired || decision.DisplayRequired {
		t.Fatal("power requests not released")
	}
}

func TestInputSequenceWrapAndScreenOnlyMode(t *testing.T) {
	var policy Policy
	options := Options{BlankDisplay: true}
	policy.Step(options, true, Input{Sequence: ^uint32(0)})
	decision := policy.Step(options, true, Input{Sequence: 2})
	if decision.SystemRequired || decision.DisplayRequired || decision.DisplayChange != 0 {
		t.Fatal(decision)
	}
	if decision = policy.Step(options, true, Input{Sequence: 2, Idle: 10 * time.Second}); decision.DisplayChange != -1 {
		t.Fatal(decision)
	}
}

type fakeSource struct{}

func (fakeSource) EnergyState() (Options, bool) {
	return Options{BlankDisplay: true, PreventSleep: true}, true
}

type fakeDevice struct {
	mu       sync.Mutex
	requests [][2]bool
	displays []bool
	applied  chan struct{}
	fail     bool
}

func (f *fakeDevice) Input() (Input, error) { return Input{Sequence: 1}, nil }
func (f *fakeDevice) Request(system, display bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, [2]bool{system, display})
	return nil
}
func (f *fakeDevice) Display(on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.displays = append(f.displays, on)
	select {
	case f.applied <- struct{}{}:
	default:
	}
	if f.fail {
		return errors.New("denied")
	}
	return nil
}

func TestManagerReleasesThreadLeaseAndWakesOnExit(t *testing.T) {
	device := &fakeDevice{applied: make(chan struct{}, 1)}
	manager := New(device, fakeSource{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); manager.Run(ctx) }()
	select {
	case <-device.applied:
	case <-time.After(2 * time.Second):
		t.Fatal("no blank request")
	}
	cancel()
	<-done
	if last := device.requests[len(device.requests)-1]; last != [2]bool{false, false} {
		t.Fatal(last)
	}
	if len(device.displays) != 2 || device.displays[0] || !device.displays[1] {
		t.Fatal(device.displays)
	}
	if !manager.Status().Supported {
		t.Fatal("missing capability")
	}
}

func TestDemoNeverTouchesDevice(t *testing.T) {
	manager := New(nil, fakeSource{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager.Run(ctx)
	if manager.Status().Supported {
		t.Fatal("demo reports native power support")
	}
}

func TestFailedScreenCommandRemainsVisibleAndIsNotRepeated(t *testing.T) {
	device := &fakeDevice{applied: make(chan struct{}, 1), fail: true}
	manager := New(device, fakeSource{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); manager.Run(ctx) }()
	select {
	case <-device.applied:
	case <-time.After(2 * time.Second):
		t.Fatal("no screen command")
	}
	time.Sleep(300 * time.Millisecond)
	if manager.Status().Warning == "" {
		t.Fatal("failure was hidden")
	}
	cancel()
	<-done
	if len(device.displays) != 2 || !device.displays[1] {
		t.Fatal("must attempt wake on cleanup without repeated off requests", device.displays)
	}
}
