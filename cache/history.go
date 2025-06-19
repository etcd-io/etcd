package cache

import clientv3 "go.etcd.io/etcd/client/v3"

// History is the storage-and-replay contract behind Cache.
type History interface {
	// Append stores one event in chronological order.
	Append(ev *clientv3.Event)
	// GetSince returns (events, latestRevision) atomically:
	// - every event with ModRevision ≥ rev that matches pred (nil = all)
	// - the current highest ModRevision kept in history.
	GetSince(rev int64, pred KeyPredicate) ([]*clientv3.Event, int64)
	// Oldest returns the first ModRevision still kept.
	OldestRevision() int64
	// LatestRevision returns the last ModRevision stored.
	LatestRevision() int64
	// RebaseHistory wipes the history and sets the new compacted revision baseline.
	RebaseHistory(newCompactedRev int64)
	// FeedFrom bulk‑copies every entry from src into the history.
	FeedFrom(src History)
}

// KeyPredicate matches etcd keys.
type KeyPredicate = func([]byte) bool
