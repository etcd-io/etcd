package matcher

type HaveLenZeroMatcher struct{}

func (HaveLenZeroMatcher) Type() Type {
	return HaveLenZeroMatcherType
}

func (HaveLenZeroMatcher) MatcherName() string {
	return haveLen
}
