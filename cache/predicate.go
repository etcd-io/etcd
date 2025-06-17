package cache

type Prefix string

func (prefix Prefix) Match(key []byte) bool {
	if prefix == "" {
		return true
	}
	prefixLen := len(prefix)
	return len(key) >= prefixLen && string(key[:prefixLen]) == string(prefix)
}
