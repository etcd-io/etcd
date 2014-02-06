package store

import (
	"testing"
)

// TestIsHidden tests isHidden functions.
func TestIsHidden(t *testing.T) {
	// watch at "/"
	// key is "/_foo", hidden to "/"
	// expected: hidden = true
	watch := "/"
	key := "/_foo"
	hidden := isHidden(watch, key)
	if !hidden {
		t.Fatalf("%v should be hidden to %v\n", key, watch)
	}

	// watch at "/_foo"
	// key is "/_foo", not hidden to "/_foo"
	// expected: hidden = false
	watch = "/_foo"
	hidden = isHidden(watch, key)
	if hidden {
		t.Fatalf("%v should not be hidden to %v\n", key, watch)
	}

	// watch at "/_foo/"
	// key is "/_foo/foo", not hidden to "/_foo"
	key = "/_foo/foo"
	hidden = isHidden(watch, key)
	if hidden {
		t.Fatalf("%v should not be hidden to %v\n", key, watch)
	}

	// watch at "/_foo/"
	// key is "/_foo/_foo", hidden to "/_foo"
	key = "/_foo/_foo"
	hidden = isHidden(watch, key)
	if !hidden {
		t.Fatalf("%v should be hidden to %v\n", key, watch)
	}

	// watch at "/_foo/foo"
	// key is "/_foo"
	watch = "_foo/foo"
	key = "/_foo/"
	hidden = isHidden(watch, key)
	if hidden {
		t.Fatalf("%v should not be hidden to %v\n", key, watch)
	}
}
