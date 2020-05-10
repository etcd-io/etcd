package foo

import "bar"

func Exported() string { return "foo" + bar.Exported() }

type t int

func (t) Method() int { return 2 }

var q int //q, yes q. The name q appears four times in this quick comment for q

func callInterface(i bar.I) {
	i.Method()
}

// q is in this comment
//q is also in this comment
