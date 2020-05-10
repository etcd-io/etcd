package pkg

import "strings"

func fn(s string) {
	_ = strings.TrimLeft(s, "")
	_ = strings.TrimLeft(s, "a")
	_ = strings.TrimLeft(s, "µ")
	_ = strings.TrimLeft(s, "abc")
	_ = strings.TrimLeft(s, "http://") // want `duplicate characters`
}
