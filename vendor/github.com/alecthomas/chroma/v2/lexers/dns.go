package lexers

import (
	"regexp"
)

// TODO(moorereason): can this be factored away?
var zoneAnalyserRe = regexp.MustCompile(`(?m)^@\s+IN\s+SOA\s+`)

func init() { // nolint: gochecknoinits
	Get("dns").SetAnalyser(func(text string) float32 {
		if zoneAnalyserRe.FindString(text) != "" {
			return 1.0
		}
		return 0.0
	})
}
