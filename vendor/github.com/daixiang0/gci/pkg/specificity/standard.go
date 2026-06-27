package specificity

type StandardMatch struct{}

func (s StandardMatch) IsMoreSpecific(than MatchSpecificity) bool {
	return isMoreSpecific(s, than)
}

func (s StandardMatch) Equal(to MatchSpecificity) bool {
	return equalSpecificity(s, to)
}

func (s StandardMatch) class() specificityClass {
	return StandardClass
}

func (s StandardMatch) String() string {
	return "Standard"
}
