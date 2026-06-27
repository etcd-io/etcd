package uv

import (
	"strings"
)

// Environ is a slice of strings that represents the environment variables of
// the program.
type Environ []string

// Getenv returns the value of the environment variable named by the key. If
// the variable is not present in the environment, the value returned will be
// the empty string.
func (p Environ) Getenv(key string) (v string) {
	v, _ = p.LookupEnv(key)
	return
}

// LookupEnv retrieves the value of the environment variable named by the key.
// If the variable is present in the environment the value (which may be empty)
// is returned and the boolean is true. Otherwise the returned value will be
// empty and the boolean will be false.
func (p Environ) LookupEnv(key string) (s string, v bool) {
	for i := len(p) - 1; i >= 0; i-- {
		if strings.HasPrefix(p[i], key+"=") {
			s = strings.TrimPrefix(p[i], key+"=")
			v = true
			break
		}
	}
	return
}
