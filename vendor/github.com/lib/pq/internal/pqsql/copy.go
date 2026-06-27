package pqsql

// StartsWithCopy reports if the SQL strings start with "copy", ignoring
// whitespace, comments, and casing.
func StartsWithCopy(query string) bool {
	if len(query) < 4 {
		return false
	}
	var linecmt, blockcmt bool
	for i := 0; i < len(query); i++ {
		c := query[i]
		if linecmt {
			linecmt = c != '\n'
			continue
		}
		if blockcmt {
			blockcmt = !(c == '/' && query[i-1] == '*')
			continue
		}
		if c == '-' && len(query) > i+1 && query[i+1] == '-' {
			linecmt = true
			continue
		}
		if c == '/' && len(query) > i+1 && query[i+1] == '*' {
			blockcmt = true
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}

		// First non-comment and non-whitespace.
		return len(query) > i+3 && c|0x20 == 'c' && query[i+1]|0x20 == 'o' &&
			query[i+2]|0x20 == 'p' && query[i+3]|0x20 == 'y'
	}
	return false
}
