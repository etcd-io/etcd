package auth

import (
	"bytes"

	"github.com/coreos/etcd/auth/authpb"
)

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

func (cs *CapturedState) HaveAcl() bool {
	return (cs.aclCache != nil) && !cs.aclCache.IsEmpty()
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

func (cs *CapturedState) CanReadWrite(path []byte, protoIdx int64, forceFindDepth int32) (bool, bool) {
	if !cs.HaveAcl() {
		return true, true
	}

	if PathIsDir(path) {
		if (cs.aclCache.GetRights(path) & uint32(authpb.BUILTIN_RIGHTS_VIEW)) != 0 {
			return true, false
		}
		if forceFindDepth > 0 {
			if pN := PathGetPrefix(path, int(forceFindDepth)); pN != nil {
				return (cs.aclCache.GetRights(pN) & uint32(authpb.BUILTIN_RIGHTS_VIEW)) != 0, false
			}
		}
	} else {
		proto := cs.GetPrototype(protoIdx)
		if proto == nil {
			return false, false
		}

		key := PathGetSuffix(path)

		field, ok := proto.Fields[string(key)]
		if !ok {
			return false, false
		}

		rights := cs.aclCache.GetRights(path)

		canRead := (field.RightsRead == 0) || ((field.RightsRead & rights) != 0)
		canWrite := (field.RightsWrite & rights) != 0

		return canRead, canWrite
	}

	return false, false
}

func PathIsRoot(key []byte) bool {
	return (len(key) == 1) && key[0] == '/'
}

func PathIsDir(key []byte) bool {
	return (len(key) > 0) && key[len(key)-1] == '/'
}

func PathGetProtoName(value []byte) []byte {
	pos := bytes.IndexByte(value, ',')
	if pos >= 0 {
		value = value[pos+1:]
		if (len(value) > 0) && (value[0] != '/') {
			return value
		}
	} else if len(value) > 0 {
		return value
	}
	return nil
}

func PathGetPrefix(key []byte, N int) []byte {
	for i := 0; i < N; i++ {
		if len(key) < 1 {
			return nil
		}
		pos := bytes.LastIndexByte(key[:len(key)-1], '/')
		if pos < 0 {
			return nil
		}
		key = key[:pos+1]
	}
	return key
}

func PathGetSuffix(key []byte) []byte {
	pos := bytes.LastIndexByte(key, '/')
	if pos < 0 {
		return key
	}
	if pos == len(key)-1 {
		return nil
	}
	return key[pos+1:]
}
