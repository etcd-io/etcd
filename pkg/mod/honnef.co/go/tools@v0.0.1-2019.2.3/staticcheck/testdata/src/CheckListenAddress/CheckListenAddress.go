package pkg

import "net/http"

func fn() {
	// Seen in actual code
	http.ListenAndServe("localhost:8080/", nil) // want `invalid port or service name in host:port pair`
	http.ListenAndServe("localhost", nil)       // want `invalid port or service name in host:port pair`
	http.ListenAndServe("localhost:8080", nil)
	http.ListenAndServe(":8080", nil)
	http.ListenAndServe(":http", nil)
	http.ListenAndServe("localhost:http", nil)
	http.ListenAndServe("local_host:8080", nil)
}
