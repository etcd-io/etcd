// Package pkg ...
package pkg

import . "fmt" // want `should not use dot imports`

var _ = Println
