package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
)

type memoryRepository struct {
	document  domain.Document
	loadError error
	saveError error
}

func (r *memoryRepository) Load() (domain.Document, error) { return r.document, r.loadError }
func (r *memoryRepository) Save(document domain.Document) error {
	r.document = document
	return r.saveError
}

type fakeObserver struct {
	process domain.Process
	sample  domain.Sample
	err     error
	entered chan struct{}
	release chan struct{}
}

func (o *fakeObserver) Processes(context.Context) ([]domain.Process, error) {
	return []domain.Process{o.process}, nil
}

func (o *fakeObserver) Read(ctx context.Context, process domain.Process) (domain.Sample, error) {
	if o.entered != nil {
		close(o.entered)
		select {
		case <-o.release:
		case <-ctx.Done():
			return domain.Sample{}, ctx.Err()
		}
	}
	return o.sample, o.err
}

type fakePower struct {
	calls int
	err   error
}

func (p *fakePower) Shutdown(context.Context) error { p.calls++; return p.err }

func fixture(t *testing.T) (*Service, *fakeObserver, *fakePower, *memoryRepository) {
	t.Helper()
	observer := &fakeObserver{process: domain.Process{PID: 42, Name: "test.exe", StartedAt: "identity"}, sample: domain.Sample{Available: true, Percent: 100, Source: "bar"}}
	power := &fakePower{}
	repository := &memoryRepository{document: domain.Document{Preferences: domain.DefaultPreferences()}}
	service, err := New(observer, power, repository, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return service, observer, power, repository
}

func readyService(t *testing.T, service *Service, observer *fakeObserver, dryRun bool) {
	t.Helper()
	if err := service.Start(context.Background(), observer.process, domain.Preferences{DelayMinutes: 1, DryRun: dryRun}); err != nil {
		t.Fatal(err)
	}
	service.current.observedDownload = true
	service.current.status.Source = "bar"
	service.current.status.Phase = "countdown"
	deadline := time.Now().Add(-time.Second)
	service.current.status.Deadline = &deadline
}

func TestPowerOnlyCalledOnceAndNeverInDryRun(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		service, observer, power, _ := fixture(t)
		readyService(t, service, observer, dryRun)
		service.poll(context.Background())
		service.poll(context.Background())
		want := 1
		if dryRun {
			want = 0
		}
		if power.calls != want || service.Status().Phase != "completed" {
			t.Fatalf("calls=%d status=%+v", power.calls, service.Status())
		}
	}
}

func TestCancellationDiscardsInflightObservation(t *testing.T) {
	service, observer, power, _ := fixture(t)
	readyService(t, service, observer, false)
	observer.entered, observer.release = make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() { defer close(done); service.poll(context.Background()) }()
	<-observer.entered
	service.Cancel()
	close(observer.release)
	<-done
	if power.calls != 0 || service.Status().Phase != "cancelled" {
		t.Fatal("cancelled session executed")
	}
}

func TestRestartRejectsOldObservation(t *testing.T) {
	service, observer, power, _ := fixture(t)
	readyService(t, service, observer, false)
	observer.entered, observer.release = make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() { defer close(done); service.poll(context.Background()) }()
	<-observer.entered
	service.Cancel()
	if err := service.Start(context.Background(), observer.process, domain.DefaultPreferences()); err != nil {
		t.Fatal(err)
	}
	close(observer.release)
	<-done
	if power.calls != 0 || service.Status().Progress != nil {
		t.Fatal("old sample affected new session")
	}
}

func TestFailuresNeverTriggerShutdown(t *testing.T) {
	service, observer, power, repository := fixture(t)
	readyService(t, service, observer, false)
	observer.err = errors.New("access denied")
	repository.saveError = errors.New("disk full")
	service.poll(context.Background())
	if power.calls != 0 || service.Status().Deadline != nil || service.Status().StorageWarning == "" {
		t.Fatal("unsafe failure handling")
	}
}

func TestPowerFailureIsReportedWithoutRetry(t *testing.T) {
	service, observer, power, _ := fixture(t)
	readyService(t, service, observer, false)
	power.err = errors.New("denied")
	service.poll(context.Background())
	service.poll(context.Background())
	if power.calls != 1 || service.Status().Phase != "error" {
		t.Fatal("failed power action retried")
	}
}

func TestStartValidatesProcessAndDelay(t *testing.T) {
	service, observer, _, _ := fixture(t)
	wrong := observer.process
	wrong.StartedAt = "reused PID"
	if service.Start(context.Background(), wrong, domain.DefaultPreferences()) == nil {
		t.Fatal("reused PID accepted")
	}
	for _, delay := range []int{0, -1, 181} {
		if service.Start(context.Background(), observer.process, domain.Preferences{DelayMinutes: delay}) == nil {
			t.Fatal("invalid delay accepted")
		}
	}
	if err := service.Start(context.Background(), observer.process, domain.DefaultPreferences()); err != nil {
		t.Fatal(err)
	}
	if service.Start(context.Background(), observer.process, domain.DefaultPreferences()) == nil {
		t.Fatal("double start accepted")
	}
}

func TestStatusIsIndependentSnapshot(t *testing.T) {
	service, observer, _, _ := fixture(t)
	readyService(t, service, observer, true)
	value := 24.0
	service.current.status.Progress = &value
	status := service.Status()
	*status.Progress = 0
	*status.Deadline = time.Time{}
	status.Events[0].Message = "modified"
	if *service.Status().Progress != 24 || service.Status().Deadline.IsZero() || service.Status().Events[0].Message == "modified" {
		t.Fatal("shared snapshot memory")
	}
}

func TestConcurrentStatusAndCancel(t *testing.T) {
	service, observer, _, _ := fixture(t)
	readyService(t, service, observer, true)
	var group sync.WaitGroup
	for i := 0; i < 20; i++ {
		group.Add(1)
		go func() { defer group.Done(); service.Status(); service.Cancel() }()
	}
	group.Wait()
}

func TestCancelledContextCannotTriggerPower(t *testing.T) {
	service, observer, power, _ := fixture(t)
	readyService(t, service, observer, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.poll(ctx)
	if power.calls != 0 {
		t.Fatal("cancelled context executed shutdown")
	}
}

func TestInvalidRepositoryFallsBackSafely(t *testing.T) {
	service, err := New(&fakeObserver{}, &fakePower{}, &memoryRepository{loadError: errors.New("broken JSON")}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	status := service.Status()
	if status.Active || !status.Preferences.DryRun || status.Preferences.DelayMinutes != 5 || status.StorageWarning == "" {
		t.Fatalf("unsafe defaults: %+v", status)
	}
}
