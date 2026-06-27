package stdlib_doclink

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"sync"

	"github.com/godoc-lint/godoc-lint/pkg/check/stdlib_doclink/internal"
)

//go:embed stdlib.json
var stdlibRaw []byte

var stdlib = sync.OnceValue(func() internal.Stdlib {
	v, _ := parseStdlib()
	return v
})

func parseStdlib() (internal.Stdlib, error) {
	result := internal.Stdlib{}
	if err := json.NewDecoder(bytes.NewReader(stdlibRaw)).Decode(&result); err != nil {
		// This never happens
		return internal.Stdlib{}, err
	}
	return result, nil
}
