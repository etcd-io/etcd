package pkg

import "strings"

func fn() {
	println(strings.Trim("\x80test\xff", "\xff")) // want `is not a valid UTF-8 encoded string`
	println(strings.Trim("foo", "bar"))

	s := "\xff"
	if true {
		s = ""
	}
	println(strings.Trim("", s))
}
