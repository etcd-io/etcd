package specificity

type NameMatch struct{}

func (n NameMatch) IsMoreSpecific(than MatchSpecificity) bool {
	return isMoreSpecific(n, than)
}

func (n NameMatch) Equal(to MatchSpecificity) bool {
	return equalSpecificity(n, to)
}

func (n NameMatch) class() specificityClass {
	return NameClass
}

func (n NameMatch) String() string {
	return "Name"
}
