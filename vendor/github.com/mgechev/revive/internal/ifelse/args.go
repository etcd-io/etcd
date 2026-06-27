package ifelse

// Args contains arguments common to the early-return, indent-error-flow,
// and superfluous-else rules.
type Args struct {
	PreserveScope bool
	AllowJump     bool
}
