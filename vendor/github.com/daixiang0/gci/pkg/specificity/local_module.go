package specificity

type LocalModule struct{}

func (m LocalModule) IsMoreSpecific(than MatchSpecificity) bool {
	return isMoreSpecific(m, than)
}

func (m LocalModule) Equal(to MatchSpecificity) bool {
	return equalSpecificity(m, to)
}

func (LocalModule) class() specificityClass {
	return LocalModuleClass
}
