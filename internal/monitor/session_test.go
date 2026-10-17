package monitor

import (
	"math"
	"testing"
	"time"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
)

func TestCompletionRequiresContinuousEvidence(t *testing.T) {
	now := time.Unix(1000, 0)
	s := newSession(domain.Process{PID: 7}, domain.Preferences{DelayMinutes: 1}, now)
	sample := func(percent float64) domain.Sample {
		return domain.Sample{Available: true, Percent: percent, Source: "bar"}
	}
	if s.advance(sample(100), now, 10*time.Second) || s.status.Deadline != nil {
		t.Fatal("initial 100 must not arm shutdown")
	}
	s.advance(sample(20), now, 10*time.Second)
	s.advance(sample(100), now, 10*time.Second)
	for second := 1; second < 60; second++ {
		if s.advance(sample(100), now.Add(time.Duration(second)*time.Second), 10*time.Second) {
			t.Fatal("shutdown before delay")
		}
	}
	if !s.advance(sample(100), now.Add(time.Minute), 10*time.Second) {
		t.Fatal("continuous 100 must complete after delay")
	}
}

func TestUnsafeSamplesResetCountdown(t *testing.T) {
	for name, sample := range map[string]domain.Sample{
		"unavailable":    {},
		"redownload":     {Available: true, Percent: 97, Source: "bar"},
		"nan":            {Available: true, Percent: math.NaN(), Source: "bar"},
		"infinite":       {Available: true, Percent: math.Inf(1), Source: "bar"},
		"negative":       {Available: true, Percent: -1, Source: "bar"},
		"overflow":       {Available: true, Percent: 101, Source: "bar"},
		"empty source":   {Available: true, Percent: 100},
		"changed source": {Available: true, Percent: 100, Source: "other"},
	} {
		t.Run(name, func(t *testing.T) {
			now := time.Now()
			s := newSession(domain.Process{}, domain.Preferences{DelayMinutes: 1}, now)
			s.advance(domain.Sample{Available: true, Percent: 20, Source: "bar"}, now, time.Minute)
			s.advance(domain.Sample{Available: true, Percent: 100, Source: "bar"}, now, time.Minute)
			if s.status.Deadline == nil {
				t.Fatal("missing countdown")
			}
			if s.advance(sample, now.Add(30*time.Second), time.Minute) || s.status.Deadline != nil {
				t.Fatal("unsafe sample did not reset countdown")
			}
		})
	}
}

func TestSuspendAndClockRollbackRestartDelay(t *testing.T) {
	for _, offset := range []time.Duration{2 * time.Minute, -time.Minute} {
		now := time.Now()
		s := newSession(domain.Process{}, domain.Preferences{DelayMinutes: 1}, now)
		s.advance(domain.Sample{Available: true, Percent: 20, Source: "bar"}, now, 10*time.Second)
		s.advance(domain.Sample{Available: true, Percent: 100, Source: "bar"}, now, 10*time.Second)
		resumed := now.Add(offset)
		if s.advance(domain.Sample{Available: true, Percent: 100, Source: "bar"}, resumed, 10*time.Second) {
			t.Fatal("must not shut down after polling gap")
		}
		if !s.status.Deadline.Equal(resumed.Add(time.Minute)) {
			t.Fatal("delay must restart")
		}
	}
}

func TestInactiveSessionCannotComplete(t *testing.T) {
	s := session{}
	if s.advance(domain.Sample{Available: true, Percent: 100, Source: "bar"}, time.Now(), time.Minute) {
		t.Fatal("inactive session completed")
	}
}
