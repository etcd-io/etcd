package pkg

import "regexp"

func fn() {
	regexp.Match(".", nil)
	regexp.MatchString(".", "")
	regexp.MatchReader(".", nil)

	for {
		regexp.Match(".", nil)       // want `calling regexp\.Match in a loop has poor performance`
		regexp.MatchString(".", "")  // want `calling regexp\.MatchString in a loop has poor performance`
		regexp.MatchReader(".", nil) // want `calling regexp\.MatchReader in a loop has poor performance`
	}
}
