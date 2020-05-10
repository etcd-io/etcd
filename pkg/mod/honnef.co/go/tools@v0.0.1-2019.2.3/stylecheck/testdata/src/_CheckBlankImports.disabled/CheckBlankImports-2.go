// Package pkg ...
package pkg

import _ "fmt" // want `blank import`

import _ "fmt" // want `blank import`
import _ "fmt"
import _ "fmt"

import _ "fmt" // want `blank import`
import "strings"
import _ "fmt" // want `blank import`

// This is fine
import _ "fmt"

// This is fine
import _ "fmt"
import _ "fmt"
import _ "fmt"

// This is fine
import _ "fmt"
import "bytes"
import _ "fmt" // want `blank import`

import _ "fmt" // This is fine

// This is not fine
import (
	_ "fmt" // want `blank import`
)

import (
	_ "fmt" // want `blank import`
	"strconv"
	// This is fine
	_ "fmt"
)

var _ = strings.NewReader
var _ = bytes.NewBuffer
var _ = strconv.IntSize
