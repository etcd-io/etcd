package doublestar

import "path/filepath"

// Validate a pattern. Patterns are validated while they run in Match(),
// PathMatch(), and Glob(), so, you normally wouldn't need to call this.
// However, there are cases where this might be useful: for example, if your
// program allows a user to enter a pattern that you'll run at a later time,
// you might want to validate it.
//
// ValidatePattern assumes your pattern uses '/' as the path separator.
func ValidatePattern(s string) bool {
	return doValidatePattern(s, '/')
}

// Like ValidatePattern, only uses your OS path separator. In other words, use
// ValidatePattern if you would normally use Match() or Glob(). Use
// ValidatePathPattern if you would normally use PathMatch(). Keep in mind,
// Glob() requires '/' separators, even if your OS uses something else.
func ValidatePathPattern(s string) bool {
	return doValidatePattern(s, filepath.Separator)
}

func doValidatePattern(s string, separator rune) bool {
	altDepth := 0
	l := len(s)
VALIDATE:
	for i := 0; i < l; i++ {
		switch s[i] {
		case '\\':
			if separator != '\\' {
				// skip the next byte - return false if there is no next byte
				if i++; i >= l {
					return false
				}
			}
			continue

		case '[':
			if i++; i >= l {
				// class didn't end
				return false
			}
			if s[i] == '^' || s[i] == '!' {
				i++
			}
			if i >= l || s[i] == ']' {
				// class didn't end or empty character class
				return false
			}

			for ; i < l; i++ {
				if separator != '\\' && s[i] == '\\' {
					i++
				} else if s[i] == ']' {
					// looks good
					continue VALIDATE
				}
			}

			// class didn't end
			return false

		case '{':
			altDepth++
			continue

		case '}':
			if altDepth == 0 {
				// alt end without a corresponding start
				return false
			}
			altDepth--
			continue
		}
	}

	// valid as long as all alts are closed
	return altDepth == 0
}
