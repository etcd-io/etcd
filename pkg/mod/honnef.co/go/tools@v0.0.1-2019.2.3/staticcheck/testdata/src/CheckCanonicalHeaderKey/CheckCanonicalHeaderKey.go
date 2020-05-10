package pkg

import "net/http"

func fn() {
	var r http.Request
	h := http.Header{}
	var m map[string][]string
	_ = h["foo"] // want `keys in http\.Header are canonicalized`
	h["foo"] = nil
	_ = r.Header["foo"] // want `keys in http\.Header are canonicalized`
	r.Header["foo"] = nil
	_ = m["foo"]
}
