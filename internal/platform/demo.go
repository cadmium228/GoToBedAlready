package platform

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
)

type Demo struct {
	mu      sync.Mutex
	started time.Time
}

func (d *Demo) Processes(context.Context) ([]domain.Process, error) {
	return []domain.Process{
		{PID: 2048, Name: "steam.exe", Title: "Steam · демонстрационная загрузка", StartedAt: "demo-steam"},
		{PID: 4096, Name: "Launcher.exe", Title: "Rockstar Games Launcher · демонстрация", StartedAt: "demo-rockstar"},
	}, nil
}

func (d *Demo) Read(ctx context.Context, process domain.Process) (domain.Sample, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started.IsZero() {
		d.started = time.Now()
	}
	percent := math.Min(100, 24+time.Since(d.started).Seconds()*1.2)
	return domain.Sample{Available: true, Percent: percent, Source: "demo-progress"}, ctx.Err()
}

type DisabledPower struct{}

func (DisabledPower) Shutdown(context.Context) error {
	return errors.New("демонстрационный режим никогда не выключает ПК")
}
