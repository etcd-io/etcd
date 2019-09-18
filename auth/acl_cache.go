package auth

import (
	"math"

	"github.com/coreos/etcd/auth/authpb"
	"github.com/coreos/etcd/pkg/adt"
)

type AclCache struct {
	Rev     int64
	entries *adt.IntervalTree
}

func NewAclCache(rev int64, entries []*authpb.AclEntry) *AclCache {
	ac := &AclCache{
		Rev: rev,
	}

	err := ac.updateInternal(entries)

	if err != nil {
		plog.Panicf("Bad acl entries: %v", err)
	}

	return ac
}

func (ac *AclCache) Update(entries []*authpb.AclEntry) (*AclCache, error) {
	newAc := &AclCache{
		Rev: ac.Rev,
	}

	err := newAc.updateInternal(entries)

	if err != nil {
		return nil, err
	}

	newAc.Rev++

	return newAc, nil
}

func (ac *AclCache) IsEmpty() bool {
	return ac.entries.Len() == 0
}

func (ac *AclCache) GetRights(path []byte) uint32 {
	if ac.IsEmpty() {
		return math.MaxUint32
	}
	rights := uint32(0)
	ac.entries.Visit(adt.NewBytesAffinePoint(path),
		func(iv *adt.IntervalValue) bool {
			val := iv.Val.(*authpb.AclEntry)
			rights |= val.RightsSet
			rights &= ^val.RightsUnset
			return true
		})
	return rights
}

func (ac *AclCache) updateInternal(acl []*authpb.AclEntry) error {
	ac.entries = &adt.IntervalTree{}

	for _, entry := range acl {
		if (len(entry.Path) > 0) && (entry.Path[len(entry.Path)-1] == '/') {
			return ErrAclBadPath
		}

		ivl := adt.NewBytesAffineInterval([]byte(entry.Path+"/"), []byte(entry.Path+"0"))
		found := false
		// For some stupid fucking reason .Find causes segfault for []byte intervals,
		// probably because of .Find uses "!=" for compare. We can't just fix .Find because it'll
		// slow down string intervals comparison
		ac.entries.Visit(ivl,
			func(iv *adt.IntervalValue) bool {
				if (iv.Ivl.Begin.Compare(ivl.Begin) == 0) && (iv.Ivl.End.Compare(ivl.End) == 0) {
					found = true
					return false
				}
				return true
			})
		if found {
			return ErrAclDuplicatePath
		}
		ac.entries.Insert(ivl, entry)
	}

	return nil
}
