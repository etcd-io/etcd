package matcher

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/nunnatsa/ginkgolinter/internal/expression/value"
	"github.com/nunnatsa/ginkgolinter/internal/gomegahandler"
)

type Type uint64

const (
	Unspecified Type = 1 << iota
	EqualMatcherType
	BeZeroMatcherType
	BeEmptyMatcherType
	BeTrueMatcherType
	BeFalseMatcherType
	BeNumericallyMatcherType
	HaveLenZeroMatcherType
	BeEquivalentToMatcherType
	BeIdenticalToMatcherType
	BeNilMatcherType
	MatchErrorMatcherType
	MultipleMatcherMatherType
	HaveValueMatherType
	WithTransformMatherType
	EqualBoolValueMatcherType
	EqualValueMatcherType
	HaveOccurredMatcherType
	SucceedMatcherType
	EqualNilMatcherType

	BoolValueFalse
	BoolValueTrue

	OrMatherType
	AndMatherType

	ErrMatchWithErr
	ErrMatchWithErrFunc
	ErrMatchWithString
	ErrMatchWithMatcher

	EqualZero
	GreaterThanZero
)

type Info interface {
	Type() Type
	MatcherName() string
}

func getMatcherInfo(orig, clone *ast.CallExpr, matcherName string, pass *analysis.Pass, handler *gomegahandler.Handler) Info {
	switch matcherName {
	case equal:
		return newEqualMatcher(orig.Args[0], clone.Args[0], pass)

	case beZero:
		return &BeZeroMatcher{}

	case beEmpty:
		return &BeEmptyMatcher{}

	case beTrue:
		return &BeTrueMatcher{}

	case beFalse:
		return &BeFalseMatcher{}

	case beNil:
		return &BeNilMatcher{}

	case beNumerically:
		if len(orig.Args) == 2 {
			return newBeNumericallyMatcher(orig.Args[0], orig.Args[1], clone.Args[1], pass)
		}

	case haveLen:
		if value.GetValuer(orig.Args[0], clone.Args[0], pass).IsValueZero() {
			return &HaveLenZeroMatcher{}
		}

	case beEquivalentTo:
		return &BeEquivalentToMatcher{
			Value: value.New(orig.Args[0], clone.Args[0], pass),
		}

	case beIdenticalTo:
		return &BeIdenticalToMatcher{
			Value: value.New(orig.Args[0], clone.Args[0], pass),
		}

	case matchError:
		return newMatchErrorMatcher(orig.Args, pass)

	case haveValue:
		if nestedMatcher := getNestedMatcher(orig, clone, 0, pass, handler); nestedMatcher != nil {
			return &HaveValueMatcher{
				nested: nestedMatcher,
			}
		}

	case withTransform:
		if nestedMatcher := getNestedMatcher(orig, clone, 1, pass, handler); nestedMatcher != nil {
			return newWithTransformMatcher(orig.Args[0], nestedMatcher, pass)
		}

	case or, and:
		matcherType := MultipleMatcherMatherType
		if matcherName == or {
			matcherType |= OrMatherType
		} else {
			matcherType |= AndMatherType
		}

		if m := newMultipleMatchersMatcher(matcherType, orig.Args, clone.Args, pass, handler); m != nil {
			return m
		}

	case succeed:
		return &SucceedMatcher{}

	case haveOccurred:
		return &HaveOccurredMatcher{}
	}

	return &UnspecifiedMatcher{matcherName: matcherName}
}

type UnspecifiedMatcher struct {
	matcherName string
}

func (UnspecifiedMatcher) Type() Type {
	return Unspecified
}

func (u UnspecifiedMatcher) MatcherName() string {
	return u.matcherName
}

func (t Type) Is(other Type) bool {
	return t&other != 0
}
