package cache

import clientv3 "go.etcd.io/etcd/client/v3"

type Prefix string

func (prefix Prefix) Match(key []byte) bool {
	if prefix == "" {
		return true
	}
	prefixLen := len(prefix)
	return len(key) >= prefixLen && string(key[:prefixLen]) == string(prefix)
}

// AfterRev builds an EntryPredicate that matches events whose ModRevision ≥ rev.
func AfterRev(rev int64) EntryPredicate {
	return func(ev *clientv3.Event) bool {
		return ev.Kv.ModRevision >= rev
	}
}
