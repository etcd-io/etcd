package pkg

import "regexp"

func fn2() string { return "" }

func fn() {
	x := "abc"
	const y = "abc"
	regexp.MustCompile(`\\.`)
	regexp.MustCompile("\\.") // want `should use raw string.+\.MustCompile`
	regexp.Compile("\\.")     // want `should use raw string.+\.Compile`
	regexp.Compile("\\.`")
	regexp.MustCompile("(?m:^lease (.+?) {\n((?s).+?)\\n}\n)")
	regexp.MustCompile("\\*/[ \t\n\r\f\v]*;")
	regexp.MustCompile(fn2())
	regexp.MustCompile(x)
	regexp.MustCompile(y)
}
