package auth

type CapturedState struct {
	prototypeCache *PrototypeCache
	aclCache       *AclCache
}

func newCapturedState(prototypeCache *PrototypeCache, aclCache *AclCache) *CapturedState {
	return &CapturedState{
		prototypeCache: prototypeCache,
		aclCache:       aclCache,
	}
}

func (cs *CapturedState) CanReadPath(path string) bool {
	// TODO: check rights, make a enum for RW checks
	return false
}
