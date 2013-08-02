package store

import (
	"path"
	"strings"
)

// keywords for internal useage
// Key for string keyword; Value for only checking prefix
var keywords = map[string]bool{
	"/_etcd":          true,
	"/ephemeralNodes": true,
}

// CheckKeyword will check if the key contains the keyword.
// For now, we only check for prefix.
func CheckKeyword(key string) bool {
	key = path.Clean("/" + key)

	// find the second "/"
	i := strings.Index(key[1:], "/")

	var prefix string

	if i == -1 {
		prefix = key
	} else {
		prefix = key[:i+1]
	}
	_, ok := keywords[prefix]

	return ok
}
