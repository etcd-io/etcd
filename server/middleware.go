package server

import (
	"net/http"

	"github.com/coreos/etcd/log"
)

type clientConnectionWatcher struct {
	next interface{}
}

func (self *clientConnectionWatcher) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var closed <-chan bool
	notifier, ok := w.(http.CloseNotifier)
	if ok {
		closed = notifier.CloseNotify()
	} else {
		// ok==false indicates the server does not have the ability
		// to tell when the client has closed. In this case, we simply
		// make a channel that is never used.
		closed = make(chan bool)
	}

	done := make(chan bool)
	go func() {
		self.next.(http.Handler).ServeHTTP(w, req)
		done <- true
	}()

	select {
	case <-done:
		return
	case <-closed:
		log.Debug("Client initiated connection close before response was ready - closing server socket")
		w.Header().Set("Connection", "close")
		return
	}
}
