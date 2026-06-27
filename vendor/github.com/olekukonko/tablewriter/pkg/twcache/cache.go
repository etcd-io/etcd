package twcache

// Cache defines a generic interface for a key-value storage with type constraints on keys and values.
// The keys must be of a type that supports comparison.
// Add inserts a new key-value pair, potentially evicting an item if necessary.
// Get retrieves a value associated with the given key, returning a boolean to indicate if the key was found.
// Purge clears all items from the cache.
type Cache[K comparable, V any] interface {
	Add(key K, value V) (evicted bool)
	Get(key K) (value V, ok bool)
	Purge()
}
