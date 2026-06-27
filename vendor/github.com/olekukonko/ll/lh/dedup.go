package lh

import (
	"sync"
	"time"

	"github.com/olekukonko/ll/lx"
)

// Dedup is a log handler that suppresses duplicate entries within a TTL window.
// It wraps another handler (H) and filters out repeated log entries that match
// within the deduplication period.
type Dedup[H lx.Handler] struct {
	next H

	ttl          time.Duration
	cleanupEvery time.Duration
	keyFn        func(*lx.Entry) uint64
	maxKeys      int

	// shards reduce lock contention by partitioning the key space
	shards [32]dedupShard

	done chan struct{}
	wg   sync.WaitGroup
	once sync.Once
}

type dedupShard struct {
	mu   sync.Mutex
	seen map[uint64]int64
}

// DedupOpt configures a Dedup handler.
type DedupOpt[H lx.Handler] func(*Dedup[H])

// WithDedupKeyFunc customizes how deduplication keys are generated.
func WithDedupKeyFunc[H lx.Handler](fn func(*lx.Entry) uint64) DedupOpt[H] {
	return func(d *Dedup[H]) { d.keyFn = fn }
}

// WithDedupCleanupInterval sets how often expired deduplication keys are purged.
func WithDedupCleanupInterval[H lx.Handler](every time.Duration) DedupOpt[H] {
	return func(d *Dedup[H]) {
		if every > 0 {
			d.cleanupEvery = every
		}
	}
}

// WithDedupMaxKeys sets a soft limit on tracked deduplication keys.
func WithDedupMaxKeys[H lx.Handler](max int) DedupOpt[H] {
	return func(d *Dedup[H]) {
		if max > 0 {
			d.maxKeys = max
		}
	}
}

// NewDedup creates a deduplicating handler wrapper.
func NewDedup[H lx.Handler](next H, ttl time.Duration, opts ...DedupOpt[H]) *Dedup[H] {
	if ttl <= 0 {
		ttl = 2 * time.Second
	}

	d := &Dedup[H]{
		next:         next,
		ttl:          ttl,
		cleanupEvery: time.Minute,
		keyFn:        defaultDedupKey,
		done:         make(chan struct{}),
	}

	// Initialize shards
	for i := 0; i < len(d.shards); i++ {
		d.shards[i].seen = make(map[uint64]int64, 64)
	}

	for _, opt := range opts {
		opt(d)
	}

	d.wg.Add(1)
	go d.cleanupLoop()

	return d
}

// Handle processes a log entry, suppressing duplicates within the TTL window.
func (d *Dedup[H]) Handle(e *lx.Entry) error {
	now := time.Now().UnixNano()
	key := d.keyFn(e)

	// Select shard based on key hash
	shardIdx := key % uint64(len(d.shards))
	shard := &d.shards[shardIdx]

	shard.mu.Lock()
	exp, ok := shard.seen[key]
	if ok && now < exp {
		shard.mu.Unlock()
		return nil
	}

	// Basic guard against unbounded growth per shard
	// Using strict limits per shard avoids global atomic counters
	limitPerShard := d.maxKeys / len(d.shards)
	if d.maxKeys > 0 && len(shard.seen) >= limitPerShard {
		// Opportunistic cleanup of current shard
		d.cleanupShard(shard, now)
	}

	shard.seen[key] = now + d.ttl.Nanoseconds()
	shard.mu.Unlock()

	return d.next.Handle(e)
}

// Close stops the cleanup goroutine and closes the underlying handler.
func (d *Dedup[H]) Close() error {
	var err error
	d.once.Do(func() {
		close(d.done)
		d.wg.Wait()

		if c, ok := any(d.next).(interface{ Close() error }); ok {
			err = c.Close()
		}
	})
	return err
}

// cleanupLoop runs periodically to purge expired deduplication keys.
func (d *Dedup[H]) cleanupLoop() {
	defer d.wg.Done()

	t := time.NewTicker(d.cleanupEvery)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			now := time.Now().UnixNano()
			// Cleanup all shards sequentially to avoid massive CPU spike
			for i := 0; i < len(d.shards); i++ {
				d.shards[i].mu.Lock()
				d.cleanupShard(&d.shards[i], now)
				d.shards[i].mu.Unlock()
			}
		case <-d.done:
			return
		}
	}
}

// cleanupShard removes expired keys from a specific shard.
func (d *Dedup[H]) cleanupShard(shard *dedupShard, now int64) {
	for k, exp := range shard.seen {
		if now > exp {
			delete(shard.seen, k)
		}
	}
}
