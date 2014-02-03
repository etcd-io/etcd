// Utility functions

package etcd

import (
	"fmt"
	"net/url"
	"reflect"
)

// Convert options to a string of HTML parameters
func optionsToString(options options, vops validOptions) (string, error) {
	p := "?"
	v := url.Values{}
	for opKey, opVal := range options {
		// Check if the given option is valid (that it exists)
		kind := vops[opKey]
		if kind == reflect.Invalid {
			return "", fmt.Errorf("Invalid option: %v", opKey)
		}

		// Check if the given option is of the valid type
		t := reflect.TypeOf(opVal)
		if kind != t.Kind() {
			return "", fmt.Errorf("Option %s should be of %v kind, not of %v kind.",
				opKey, kind, t.Kind())
		}

		v.Set(opKey, fmt.Sprintf("%v", opVal))
	}
	p += v.Encode()
	return p, nil
}
