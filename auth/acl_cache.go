package auth

import (
	"github.com/coreos/etcd/auth/authpb"
)

type AclCache struct {
	rev int64
	// TODO: use interval tree for fast access
}

func newAclCache(rev int64) *AclCache {
	ac := &AclCache{
		rev: rev,
	}

	// TODO: fill in the cache

	return ac
}

func (ac *AclCache) Update(acl []*authpb.AclEntry) *AclCache {
	// TODO: update, return new cache if changed
	return nil
}
