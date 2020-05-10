// +build one two three go1.1
// +build three one two go1.1

package pkg // want `identical build constraints`
