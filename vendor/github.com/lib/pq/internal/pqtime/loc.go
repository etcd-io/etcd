package pqtime

import (
	"sync"
	"time"
)

// The location cache caches the time zones typically used by the client.
type locationCache struct {
	cache map[int]*time.Location
	lock  sync.Mutex
}

// All connections share the same list of timezones. Benchmarking shows that
// about 5% speed could be gained by putting the cache in the connection and
// losing the mutex, at the cost of a small amount of memory and a somewhat
// significant increase in code complexity.
var globalLocationCache = &locationCache{cache: make(map[int]*time.Location)}

func Reset() {
	globalLocationCache = &locationCache{cache: make(map[int]*time.Location)}
}

// Returns the cached timezone for the specified offset, creating and caching
// it if necessary.
func (c *locationCache) getLocation(offset int) *time.Location {
	c.lock.Lock()
	defer c.lock.Unlock()
	l, ok := c.cache[offset]
	if !ok {
		// TODO(v2): for offset=0 it should use some descriptive text like
		// "without time zone".
		l = time.FixedZone("", offset)
		c.cache[offset] = l
	}
	return l
}
