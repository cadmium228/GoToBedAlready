package monitor

import (
	"time"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
)

type session struct {
	status           domain.Status
	observedDownload bool
	lastSample       time.Time
}

func newSession(process domain.Process, preferences domain.Preferences, now time.Time) session {
	return session{status: domain.Status{
		Phase: "waiting", Active: true, Process: process, Preferences: preferences,
		Message: "Ожидаем доступный прогресс ниже 100%", UpdatedAt: now,
	}}
}

func (s *session) advance(sample domain.Sample, now time.Time, maxGap time.Duration) bool {
	if !s.status.Active {
		return false
	}
	s.status.UpdatedAt = now
	if !s.lastSample.IsZero() && (now.Sub(s.lastSample) > maxGap || now.Before(s.lastSample)) {
		s.status.Deadline = nil
	}
	s.lastSample = now
	if !sample.Valid() {
		s.status.Phase = "waiting"
		s.status.Progress = nil
		s.status.Deadline = nil
		s.status.Message = "Прогресс недоступен. Выключение заблокировано"
		if sample.Detail != "" {
			s.status.Message += ": " + sample.Detail
		}
		return false
	}
	if sample.Source != s.status.Source {
		s.observedDownload = false
		s.status.Deadline = nil
		s.status.Source = sample.Source
	}
	progress := sample.Percent
	s.status.Progress = &progress
	if progress < 100 {
		s.observedDownload = true
		s.status.Deadline = nil
		s.status.Phase = "downloading"
		s.status.Message = "Следим за загрузкой. Можно отдыхать"
		return false
	}
	if !s.observedDownload {
		s.status.Phase = "waiting"
		s.status.Message = "Уже 100%. Для безопасности сначала нужен прогресс ниже 100%"
		return false
	}
	if s.status.Deadline == nil {
		deadline := now.Add(time.Duration(s.status.Preferences.DelayMinutes) * time.Minute)
		s.status.Deadline = &deadline
	}
	s.status.Phase = "countdown"
	s.status.Message = "100% подтверждены. Проверяем, не начнётся ли докачивание"
	return !now.Before(*s.status.Deadline)
}
