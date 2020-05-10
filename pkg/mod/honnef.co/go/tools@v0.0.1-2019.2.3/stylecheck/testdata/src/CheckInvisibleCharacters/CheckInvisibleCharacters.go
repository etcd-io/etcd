// Package pkg ...
package pkg

var (
	a = ""  // want `Unicode control character U\+0007`
	b = "" // want `Unicode control character U\+0007` `Unicode control character U\+001A`
	c = "Test	test"
	d = `T
est`
	e = `Zero​Width` // want `Unicode format character U\+200B`
	f = "\u200b"
)
