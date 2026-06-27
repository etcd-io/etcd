package tw

// State represents an on/off state
type State int

// Public: Methods for State type

// Enabled checks if the state is on
func (o State) Enabled() bool { return o == Success }

// Default checks if the state is unknown
func (o State) Default() bool { return o == Unknown }

// Disabled checks if the state is off
func (o State) Disabled() bool { return o == Fail }

// Toggle switches the state between on and off
func (o State) Toggle() State {
	if o == Fail {
		return Success
	}
	return Fail
}

// Cond executes a condition if the state is enabled
func (o State) Cond(c func() bool) bool {
	if o.Enabled() {
		return c()
	}
	return false
}

// Or returns this state if enabled, else the provided state
func (o State) Or(c State) State {
	if o.Enabled() {
		return o
	}
	return c
}

// String returns the string representation of the state
func (o State) String() string {
	if o.Enabled() {
		return "on"
	}

	if o.Disabled() {
		return "off"
	}
	return "undefined"
}
