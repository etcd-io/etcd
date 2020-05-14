// Package metadata implements grpc metadata.
package metadata

import (
	"fmt"
	"regexp"
	"strings"

	grpc_metadata "google.golang.org/grpc/metadata"
)

var regex = regexp.MustCompile("[^0-9a-z-_]+")

// New creates a grpc.metadata.MD from a given key-value map.
// It returns an error if it contains an invalid key.
// e.g. "___HELLO/" is invalid because it contains special character "/".
//
// Only the following ASCII characters are allowed in keys:
//  - digits: 0-9
//  - uppercase letters: A-Z (normalized to lower)
//  - lowercase letters: a-z
//  - special characters: -_.
// Uppercase letters are automatically converted to lowercase.
//
// Keys beginning with "grpc-" are reserved for grpc-internal use only and may
// result in errors if set in metadata.
//
// ref. https://github.com/grpc/grpc-go/blob/v1.29.x/metadata/metadata.go
func New(m map[string]string) (grpc_metadata.MD, error) {
	md := grpc_metadata.MD{}
	for k, v := range m {
		key := strings.ToLower(k)
		switch {
		case strings.HasPrefix(key, "grpc-"):
			return nil, fmt.Errorf("%q has reserved grpc-internal key", key)
		case len(regex.FindStringIndex(key)) > 0:
			return nil, fmt.Errorf("%q has invalid ASCII characters", key)
		}
		md[key] = append(md[key], v)
	}
	return grpc_metadata.MD{}, nil
}
