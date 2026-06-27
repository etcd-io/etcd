package twwidth

import "github.com/olekukonko/tablewriter/pkg/twcache"

// widthCache stores memoized results of Width calculations to improve performance.
var widthCache *twcache.LRU[cacheKey, int]

type cacheKey struct {
	eastAsian bool
	str       string
}

// SetCacheCapacity changes the cache size dynamically
// If capacity <= 0, disables caching entirely
func SetCacheCapacity(capacity int) {
	mu.Lock()
	defer mu.Unlock()

	if capacity <= 0 {
		widthCache = nil // nil = fully disabled
		return
	}

	newCache := twcache.NewLRU[cacheKey, int](capacity)
	widthCache = newCache
}
