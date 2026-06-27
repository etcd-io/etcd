package matcher

import "github.com/nunnatsa/ginkgolinter/internal/expression/value"

type BeIdenticalToMatcher struct {
	value.Value
}

func (BeIdenticalToMatcher) Type() Type {
	return BeIdenticalToMatcherType
}

func (BeIdenticalToMatcher) MatcherName() string {
	return beIdenticalTo
}

type BeEquivalentToMatcher struct {
	value.Value
}

func (BeEquivalentToMatcher) Type() Type {
	return BeEquivalentToMatcherType
}

func (BeEquivalentToMatcher) MatcherName() string {
	return beEquivalentTo
}

type BeZeroMatcher struct{}

func (BeZeroMatcher) Type() Type {
	return BeZeroMatcherType
}

func (BeZeroMatcher) MatcherName() string {
	return beZero
}

type BeEmptyMatcher struct{}

func (BeEmptyMatcher) Type() Type {
	return BeEmptyMatcherType
}

func (BeEmptyMatcher) MatcherName() string {
	return beEmpty
}

type BeTrueMatcher struct{}

func (BeTrueMatcher) Type() Type {
	return BeTrueMatcherType | BoolValueTrue
}

func (BeTrueMatcher) MatcherName() string {
	return beTrue
}

type BeFalseMatcher struct{}

func (BeFalseMatcher) Type() Type {
	return BeFalseMatcherType | BoolValueFalse
}

func (BeFalseMatcher) MatcherName() string {
	return beFalse
}

type BeNilMatcher struct{}

func (BeNilMatcher) Type() Type {
	return BeNilMatcherType
}

func (BeNilMatcher) MatcherName() string {
	return beNil
}
