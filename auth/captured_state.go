package auth

type CapturedState struct {
	ProtoCache *PrototypeCache
	AclCache   *AclCache
}

func NewCapturedState(prototypeCache *PrototypeCache, aclCache *AclCache) *CapturedState {
	return &CapturedState{
		ProtoCache: prototypeCache,
		AclCache:   aclCache,
	}
}
