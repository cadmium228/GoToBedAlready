package energy

import "time"

type Options struct {
	BlankDisplay bool `json:"blankDisplay"`
	PreventSleep bool `json:"preventSleep"`
}

type Input struct {
	Sequence uint32
	Idle     time.Duration
}

type Decision struct {
	SystemRequired  bool
	DisplayRequired bool
	DisplayChange   int
}

type Policy struct {
	engaged   bool
	blanked   bool
	lastInput uint32
}

func (p *Policy) Step(options Options, monitoring bool, input Input) Decision {
	eligible := options.BlankDisplay && monitoring
	decision := Decision{SystemRequired: options.PreventSleep, DisplayRequired: options.PreventSleep}
	if !eligible {
		if p.blanked {
			decision.DisplayChange = 1
		}
		p.engaged, p.blanked = false, false
		return decision
	}
	if !p.engaged {
		p.engaged, p.blanked, p.lastInput = true, true, input.Sequence
		decision.DisplayChange = -1
	} else {
		if input.Sequence != p.lastInput {
			p.lastInput = input.Sequence
			p.blanked = false
		}
		if !p.blanked && input.Idle >= 10*time.Second {
			p.blanked = true
			decision.DisplayChange = -1
		}
	}
	decision.DisplayRequired = options.PreventSleep && !p.blanked
	return decision
}
