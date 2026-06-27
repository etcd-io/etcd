package wsl

import (
	"fmt"
	"strings"
)

// CheckSet is a set of checks to run.
type CheckSet map[CheckType]struct{}

// CheckType is a type that represents a checker to run.
type CheckType int

// Each checker is represented by a CheckType that is used to enable or disable
// the check.
// A check can either be of a specific built-in keyword or custom checks.
const (
	CheckInvalid CheckType = iota
	CheckAssign
	CheckBranch
	CheckDecl
	CheckDefer
	CheckExpr
	CheckFor
	CheckGo
	CheckIf
	CheckIncDec
	CheckLabel
	CheckRange
	CheckReturn
	CheckSelect
	CheckSend
	CheckSwitch
	CheckTypeSwitch

	// CheckAfterBlock ensures there's a newline after each block.
	CheckAfterBlock
	// CheckAfterDecl ensures there's a newline after a declaration statement
	// (`var`, `const`, `type`) unless the following statement is another
	// declaration.
	CheckAfterDecl
	// CheckAfterDefer ensures there's a newline after a `defer` statement
	// unless the following statement is another `defer`.
	CheckAfterDefer
	// CheckAfterExpr ensures there's a newline after an expression statement
	// unless the following statement is another expression statement.
	CheckAfterExpr
	// CheckAfterGo ensures there's a newline after a `go` statement unless the
	// following statement is another `go`.
	CheckAfterGo
	// CheckAppend only allows assignments of `append` to be cuddled with other
	// assignments if it's a variable used in the append statement, e.g.
	//
	// a := 1
	// x = append(x, a)
	// .
	CheckAppend
	// CheckAssignExclusive only allows assignments of either new variables or
	// re-assignment of existing ones, e.g.
	//
	// a := 1
	// b := 2
	//
	// a = 1
	// b = 2
	// .
	CheckAssignExclusive
	// CheckAssignExpr will check so assignments are not cuddled with expression
	// nodes, e.g.
	//
	// t1.Fn1()
	//
	// x := t1.Fn2()
	// t1.Fn3()
	// .
	CheckAssignExpr
	// CheckCuddleGroup changes how cuddle-max-statements violations are
	// reported when more than the configured number of cuddled statements
	// share a variable with the trigger statement (e.g. `if`, `for`,
	// `switch`). Instead of pointing at the (N+1)th cuddled statement and
	// splitting the cuddled group, the diagnostic is placed on the trigger
	// itself so the entire cuddled group stays together and gets separated
	// from the trigger by a blank line, e.g.
	//
	// a := 1
	// b := 2
	//
	// if a > b {}
	// .
	CheckCuddleGroup
	// CheckErr force error checking to follow immediately after an error
	// variable is assigned, e.g.
	//
	// _, err := someFn()
	// if err != nil {
	//     panic(err)
	// }
	// .
	CheckErr
	CheckLeadingWhitespace
	CheckTrailingWhitespace

	//nolint:godoclint // No need to document
	// CheckTypes only used for reporting.
	CheckCaseTrailingNewline
)

func (c CheckType) String() string {
	return [...]string{
		"invalid",
		"assign",
		"branch",
		"decl",
		"defer",
		"expr",
		"for",
		"go",
		"if",
		"inc-dec",
		"label",
		"range",
		"return",
		"select",
		"send",
		"switch",
		"type-switch",
		//
		"after-block",
		"after-decl",
		"after-defer",
		"after-expr",
		"after-go",
		"append",
		"assign-exclusive",
		"assign-expr",
		"cuddle-group",
		"err",
		"leading-whitespace",
		"trailing-whitespace",
		//
		"case-trailing-newline",
	}[c]
}

type Configuration struct {
	IncludeGenerated    bool
	AllowFirstInBlock   bool
	AllowWholeBlock     bool
	BranchMaxLines      int
	CaseMaxLines        int
	CuddleMaxStatements int
	Checks              CheckSet
}

func NewConfig() *Configuration {
	return &Configuration{
		IncludeGenerated:    false,
		AllowFirstInBlock:   true,
		AllowWholeBlock:     false,
		CaseMaxLines:        0,
		BranchMaxLines:      2,
		CuddleMaxStatements: 1,
		Checks:              DefaultChecks(),
	}
}

func NewWithChecks(
	defaultChecks string,
	enable []string,
	disable []string,
) (*Configuration, error) {
	checks, err := NewCheckSet(defaultChecks, enable, disable)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	cfg := NewConfig()
	cfg.Checks = checks

	return cfg, nil
}

func NewCheckSet(
	defaultChecks string,
	enable []string,
	disable []string,
) (CheckSet, error) {
	var cs CheckSet

	switch strings.ToLower(defaultChecks) {
	case "", "default":
		cs = DefaultChecks()
	case "all":
		cs = AllChecks()
	case "none":
		cs = NoChecks()
	default:
		return nil, fmt.Errorf("invalid preset '%s', must be `all`, `none` or `` (empty)", defaultChecks)
	}

	for _, s := range enable {
		check, err := CheckFromString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid check '%s'", s)
		}

		cs.Add(check)
	}

	for _, s := range disable {
		check, err := CheckFromString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid check '%s'", s)
		}

		cs.Remove(check)
	}

	return cs, nil
}

func DefaultChecks() CheckSet {
	return CheckSet{
		CheckAppend:             {},
		CheckAssign:             {},
		CheckBranch:             {},
		CheckDecl:               {},
		CheckDefer:              {},
		CheckErr:                {},
		CheckExpr:               {},
		CheckFor:                {},
		CheckGo:                 {},
		CheckIf:                 {},
		CheckIncDec:             {},
		CheckLabel:              {},
		CheckLeadingWhitespace:  {},
		CheckTrailingWhitespace: {},
		CheckRange:              {},
		CheckReturn:             {},
		CheckSelect:             {},
		CheckSend:               {},
		CheckSwitch:             {},
		CheckTypeSwitch:         {},
	}
}

func AllChecks() CheckSet {
	c := DefaultChecks()
	c.Add(CheckAssignExclusive)
	c.Add(CheckAssignExpr)
	c.Add(CheckAfterBlock)
	c.Add(CheckAfterDecl)
	c.Add(CheckAfterDefer)
	c.Add(CheckAfterExpr)
	c.Add(CheckAfterGo)
	c.Add(CheckCuddleGroup)

	return c
}

func NoChecks() CheckSet {
	return CheckSet{}
}

func (c CheckSet) Add(check CheckType) {
	c[check] = struct{}{}
}

func (c CheckSet) Remove(check CheckType) {
	delete(c, check)
}

func CheckFromString(s string) (CheckType, error) {
	switch strings.ToLower(s) {
	case "assign":
		return CheckAssign, nil
	case "branch":
		return CheckBranch, nil
	case "decl":
		return CheckDecl, nil
	case "defer":
		return CheckDefer, nil
	case "expr":
		return CheckExpr, nil
	case "for":
		return CheckFor, nil
	case "go":
		return CheckGo, nil
	case "if":
		return CheckIf, nil
	case "inc-dec":
		return CheckIncDec, nil
	case "label":
		return CheckLabel, nil
	case "range":
		return CheckRange, nil
	case "return":
		return CheckReturn, nil
	case "select":
		return CheckSelect, nil
	case "send":
		return CheckSend, nil
	case "switch":
		return CheckSwitch, nil
	case "type-switch":
		return CheckTypeSwitch, nil

	case "after-block":
		return CheckAfterBlock, nil
	case "after-decl":
		return CheckAfterDecl, nil
	case "after-defer":
		return CheckAfterDefer, nil
	case "after-expr":
		return CheckAfterExpr, nil
	case "after-go":
		return CheckAfterGo, nil
	case "append":
		return CheckAppend, nil
	case "assign-exclusive":
		return CheckAssignExclusive, nil
	case "assign-expr":
		return CheckAssignExpr, nil
	case "err":
		return CheckErr, nil
	case "cuddle-group":
		return CheckCuddleGroup, nil
	case "leading-whitespace":
		return CheckLeadingWhitespace, nil
	case "trailing-whitespace":
		return CheckTrailingWhitespace, nil
	default:
		return CheckInvalid, fmt.Errorf("invalid check '%s'", s)
	}
}
