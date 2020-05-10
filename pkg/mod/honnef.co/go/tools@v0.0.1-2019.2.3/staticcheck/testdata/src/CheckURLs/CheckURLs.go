package pkg

import "net/url"

func fn() {
	url.Parse("foobar")
	url.Parse(":") // want `is not a valid URL`
	url.Parse("https://golang.org")
}
