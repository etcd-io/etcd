package txn

import (
	mrand "math/rand"
	"time"
)

var chaosEnabled = false
var chaosSetting Chaos

// Chaos holds parameters for the failure injection mechanism.
type Chaos struct {
	// KillChance is the 0.0 to 1.0 chance that a given checkpoint
	// within the algorithm will raise an interruption that will
	// stop the procedure.
	KillChance float64

	// SlowdownChance is the 0.0 to 1.0 chance that a given checkpoint
	// within the algorithm will be delayed by Slowdown before
	// continuing.
	SlowdownChance float64
	Slowdown       time.Duration

	// If Breakpoint is set, the above settings will only affect the
	// named breakpoint.
	Breakpoint string
}

// SetChaos sets the failure injection parameters to c.
func SetChaos(c Chaos) {
	chaosSetting = c
	chaosEnabled = c.KillChance > 0 || c.SlowdownChance > 0
}

func chaos(bpname string) {
	if !chaosEnabled {
		return
	}
	switch chaosSetting.Breakpoint {
	case "", bpname:
		kc := chaosSetting.KillChance
		if kc > 0 && mrand.Intn(1000) < int(kc*1000) {
			panic(chaosError{})
		}
		if bpname == "insert" {
			return
		}
		sc := chaosSetting.SlowdownChance
		if sc > 0 && mrand.Intn(1000) < int(sc*1000) {
			time.Sleep(chaosSetting.Slowdown)
		}
	}
}

type chaosError struct{}

func (f *flusher) handleChaos(err *error) {
	v := recover()
	if v == nil {
		return
	}
	if _, ok := v.(chaosError); ok {
		f.debugf("Killed by chaos!")
		*err = ErrChaos
		return
	}
	panic(v)
}
