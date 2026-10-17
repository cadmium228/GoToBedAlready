package domain

import (
	"errors"
	"math"
	"time"
)

type Process struct {
	PID       int    `json:"pid"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	StartedAt string `json:"startedAt"`
}

type Sample struct {
	Available bool    `json:"available"`
	Percent   float64 `json:"percent"`
	Source    string  `json:"source"`
	Detail    string  `json:"detail"`
}

func (s Sample) Valid() bool {
	return s.Available && s.Source != "" && !math.IsNaN(s.Percent) && !math.IsInf(s.Percent, 0) && s.Percent >= 0 && s.Percent <= 100
}

type Preferences struct {
	DelayMinutes int  `json:"delayMinutes"`
	DryRun       bool `json:"dryRun"`
	BlankDisplay bool `json:"blankDisplay"`
	PreventSleep bool `json:"preventSleep"`
}

func DefaultPreferences() Preferences {
	return Preferences{DelayMinutes: 5, DryRun: true, PreventSleep: true}
}

func (p Preferences) Validate() error {
	if p.DelayMinutes < 1 || p.DelayMinutes > 180 {
		return errors.New("задержка должна быть от 1 до 180 минут")
	}
	return nil
}

type Event struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

type Document struct {
	Preferences Preferences `json:"preferences"`
	Events      []Event     `json:"events"`
}

type Status struct {
	Phase          string      `json:"phase"`
	Active         bool        `json:"active"`
	Process        Process     `json:"process"`
	Preferences    Preferences `json:"preferences"`
	Progress       *float64    `json:"progress"`
	Source         string      `json:"source"`
	Message        string      `json:"message"`
	Deadline       *time.Time  `json:"deadline"`
	UpdatedAt      time.Time   `json:"updatedAt"`
	Events         []Event     `json:"events"`
	StorageWarning string      `json:"storageWarning"`
}
