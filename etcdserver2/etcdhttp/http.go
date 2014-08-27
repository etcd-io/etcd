package etcdhttp

import (
	"io"
	"net/http"
	"time"

	"code.google.com/p/go.net/context"
	etcdserver "github.com/coreos/etcd/etcdserver2"
	"github.com/coreos/etcd/raft"
)

func SendWithPrefix(prefix string, send etcdserver.SendFunc) etcdserver.SendFunc {
	return etcdserver.SendFunc(func(m []raft.Message) {
		/*
			url = parseurl
			u.Path = prefix + u.Path
			for maxTrys {
				resp, err := http.Post(u.String(), ...)
				if err...
					backoff?
			}
		*/
	})
}

const DefaultTimeout = 500 * time.Millisecond

type Handler struct {
	Timeout time.Duration
	Server  etcdserver.Server
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: set read/write timeout?

	timeout := h.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, _ := context.WithTimeout(context.Background(), timeout)
	// TODO(bmizerany): watch the closenotify chan in another goroutine can
	// call cancel when it closes. be sure to watch ctx.Done() too so we
	// don't leak a ton of these goroutines.

	rr, err := parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	resp, err := h.Server.Do(ctx, rr)
	if err != nil {
		// TODO(bmizerany): switch on store errors and etcdserver.ErrUnknownMethod
		panic("TODO")
	}

	encodeResponse(w, resp)
}

func parseRequest(r *http.Request) (etcdserver.Request, error) {
	return etcdserver.Request{}, nil
}

func encodeResponse(w io.Writer, resp etcdserver.Response) {

}
