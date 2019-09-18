package auth

type CapturedState struct {
	protoCache *PrototypeCache
	aclCache   *AclCache
}

func NewCapturedState(prototypeCache *PrototypeCache, aclCache *AclCache) *CapturedState {
	return &CapturedState{
		protoCache: prototypeCache,
		aclCache:   aclCache,
	}
}

func (cs *CapturedState) IsRoot() bool {
	return cs.aclCache == nil
}

func (cs *CapturedState) GetPrototypeByName(name string) *CachedPrototype {
	if cs.protoCache == nil {
		return nil
	}
	return cs.protoCache.GetPrototypeByName(name)
}

func (cs *CapturedState) GetPrototype(idx int64) *CachedPrototype {
	if cs.protoCache == nil {
		return nil
	}
	return cs.protoCache.GetPrototype(idx)
}

func (cs *CapturedState) CheckRights(path []byte) (*CachedPrototype, bool, bool) {
	return nil, false, false
}
