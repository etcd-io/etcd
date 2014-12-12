package etcdmain

import (
	"io/ioutil"
	"log"
	"net"
	"net/http"
)

// serveHTTP accepts incoming HTTP connections on the listener l,
// creating a new service goroutine for each. The service goroutines
// read requests and then call handler to reply to them.
func serveHTTP(l net.Listener, handler http.Handler) error {
	logger := log.New(ioutil.Discard, "etcdhttp", 0)
	// TODO: add debug flag; enable logging when debug flag is set
	srv := &http.Server{
		Handler:  handler,
		ErrorLog: logger, // do not log user error
	}
	return srv.Serve(l)
}
