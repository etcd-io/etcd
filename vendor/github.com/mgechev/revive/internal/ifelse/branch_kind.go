package ifelse

// BranchKind is a classifier for if-else branches. It says whether the branch is empty,
// and whether the branch ends with a statement that deviates control flow.
type BranchKind int

const (
	// Empty branches do nothing.
	Empty BranchKind = iota

	// Return branches return from the current function.
	Return

	// Continue branches continue a surrounding "for" loop.
	Continue

	// Break branches break a surrounding "for" loop.
	Break

	// Goto branches conclude with a "goto" statement.
	Goto

	// Panic branches panic the current function.
	Panic

	// Exit branches end the program.
	Exit

	// Regular branches do not fit any category above.
	Regular
)

// IsEmpty tests if the branch is empty.
func (k BranchKind) IsEmpty() bool { return k == Empty }

// Returns tests if the branch returns from the current function.
func (k BranchKind) Returns() bool { return k == Return }

// Deviates tests if the control does not flow to the first
// statement following the if-else chain.
func (k BranchKind) Deviates() bool {
	switch k {
	case Empty, Regular:
		return false
	case Return, Continue, Break, Goto, Panic, Exit:
		return true
	}
	panic("invalid kind")
}

// Branch returns a Branch with the given kind.
func (k BranchKind) Branch() Branch { return Branch{BranchKind: k} }

// String returns a brief string representation.
func (k BranchKind) String() string {
	switch k {
	case Empty, Regular:
		return ""
	case Return:
		return "return"
	case Continue:
		return "continue"
	case Break:
		return "break"
	case Goto:
		return "goto"
	case Panic:
		return "panic()"
	case Exit:
		return "os.Exit()"
	}
	panic("invalid kind")
}

// LongString returns a longer form string representation.
func (k BranchKind) LongString() string {
	switch k {
	case Empty:
		return "an empty block"
	case Regular:
		return "a regular statement"
	case Return:
		return "a return statement"
	case Continue:
		return "a continue statement"
	case Break:
		return "a break statement"
	case Goto:
		return "a goto statement"
	case Panic:
		return "a function call that panics"
	case Exit:
		return "a function call that exits the program"
	}
	panic("invalid kind")
}
