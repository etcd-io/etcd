package gosec

import "regexp"

// regexCacheKey is the cache key for regex match results.
type regexCacheKey struct {
	Re  *regexp.Regexp
	Str string
}

// RegexMatchWithCache returns the result of re.MatchString(s), using GlobalCache
// to store previous results for improved performance on repeated lookups.
func RegexMatchWithCache(re *regexp.Regexp, s string) bool {
	key := regexCacheKey{Re: re, Str: s}
	if val, ok := GlobalCache.Get(key); ok {
		return val.(bool)
	}
	res := re.MatchString(s)
	GlobalCache.Add(key, res)
	return res
}
