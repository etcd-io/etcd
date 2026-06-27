package formatter

import (
	"bytes"
	"encoding/json"

	"github.com/mgechev/revive/lint"
)

// NDJSON is an implementation of the [lint.Formatter] interface
// which formats the errors to NDJSON stream.
type NDJSON struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter.
func (*NDJSON) Name() string {
	return "ndjson"
}

// Format formats the failures gotten from the lint.
func (*NDJSON) Format(failures <-chan lint.Failure, config lint.Config) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for failure := range failures {
		obj := jsonObject{}
		obj.Severity = severity(config, failure)
		obj.Failure = failure
		err := enc.Encode(obj)
		if err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}
