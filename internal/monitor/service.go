package monitor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
)

type Observer interface {
	Processes(context.Context) ([]domain.Process, error)
	Read(context.Context, domain.Process) (domain.Sample, error)
}

type PowerController interface {
	Shutdown(context.Context) error
}

type Repository interface {
	Load() (domain.Document, error)
	Save(domain.Document) error
}

type Service struct {
	mu         sync.Mutex
	observer   Observer
	power      PowerController
	repository Repository
	document   domain.Document
	current    session
	generation uint64
	interval   time.Duration
	wake       chan struct{}
}

func New(observer Observer, power PowerController, repository Repository, interval time.Duration) (*Service, error) {
	if interval <= 0 {
		return nil, errors.New("интервал опроса должен быть положительным")
	}
	document, err := repository.Load()
	warning := ""
	if err != nil {
		document = domain.Document{Preferences: domain.DefaultPreferences()}
		warning = "Не удалось прочитать настройки: " + err.Error()
	}
	if err := document.Preferences.Validate(); err != nil {
		document.Preferences = domain.DefaultPreferences()
		warning = "Некорректные настройки сброшены"
	}
	if len(document.Events) > 50 {
		document.Events = document.Events[len(document.Events)-50:]
	}
	service := &Service{observer: observer, power: power, repository: repository, document: document, interval: interval, wake: make(chan struct{}, 1)}
	service.current.status = domain.Status{Phase: "idle", Preferences: document.Preferences, Message: "Выберите процесс и начните наблюдение", StorageWarning: warning}
	return service, nil
}

func (s *Service) Processes(ctx context.Context) ([]domain.Process, error) {
	return s.observer.Processes(ctx)
}

func (s *Service) Start(ctx context.Context, process domain.Process, preferences domain.Preferences) error {
	if err := preferences.Validate(); err != nil {
		return err
	}
	processes, err := s.observer.Processes(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, candidate := range processes {
		if candidate.PID == process.PID && candidate.StartedAt == process.StartedAt {
			process = candidate
			found = true
			break
		}
	}
	if !found {
		return errors.New("процесс завершён или перезапущен; выберите его заново")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current.status.Active {
		return errors.New("наблюдение уже запущено")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.generation++
	s.document.Preferences = preferences
	s.current = newSession(process, preferences, time.Now())
	s.recordLocked("Наблюдение: " + process.Name)
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}

func (s *Service) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.current.status.Active {
		return
	}
	s.generation++
	s.current.status.Active = false
	s.current.status.Deadline = nil
	s.current.status.Phase = "cancelled"
	s.current.status.Message = "Наблюдение отменено. ПК останется включённым"
	s.recordLocked(s.current.status.Message)
}

func (s *Service) Status() domain.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.current.status
	status.Events = append([]domain.Event{}, s.document.Events...)
	if status.Progress != nil {
		value := *status.Progress
		status.Progress = &value
	}
	if status.Deadline != nil {
		deadline := *status.Deadline
		status.Deadline = &deadline
	}
	return status
}

func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	defer s.Cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		}
		s.poll(ctx)
	}
}

func (s *Service) poll(ctx context.Context) {
	s.mu.Lock()
	if !s.current.status.Active {
		s.mu.Unlock()
		return
	}
	generation, process := s.generation, s.current.status.Process
	s.mu.Unlock()
	readContext, cancel := context.WithTimeout(ctx, 12*time.Second)
	sample, err := s.observer.Read(readContext, process)
	cancel()
	if err != nil {
		sample = domain.Sample{Detail: err.Error()}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.generation || !s.current.status.Active || ctx.Err() != nil {
		return
	}
	previousPhase := s.current.status.Phase
	ready := s.current.advance(sample, time.Now(), s.interval+15*time.Second)
	if previousPhase != s.current.status.Phase {
		s.recordLocked(s.current.status.Message)
	}
	if !ready {
		return
	}
	s.current.status.Active = false
	s.current.status.Deadline = nil
	if s.current.status.Preferences.DryRun {
		s.current.status.Phase = "completed"
		s.current.status.Message = "Проверка пройдена. В тестовом режиме ПК не выключается"
	} else {
		shutdownContext, stop := context.WithTimeout(ctx, 10*time.Second)
		err := s.power.Shutdown(shutdownContext)
		stop()
		if err != nil {
			s.current.status.Phase = "error"
			s.current.status.Message = fmt.Sprintf("Не удалось выключить ПК: %v", err)
		} else {
			s.current.status.Phase = "completed"
			s.current.status.Message = "Команда выключения передана Windows"
		}
	}
	s.recordLocked(s.current.status.Message)
}

func (s *Service) recordLocked(message string) {
	s.document.Events = append(s.document.Events, domain.Event{Time: time.Now(), Message: message})
	if len(s.document.Events) > 50 {
		s.document.Events = s.document.Events[len(s.document.Events)-50:]
	}
	if err := s.repository.Save(s.document); err != nil {
		s.current.status.StorageWarning = "Не удалось сохранить настройки: " + err.Error()
	} else {
		s.current.status.StorageWarning = ""
	}
}
