package debugutil

import (
	"expvar"
	"fmt"
	"github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
	"go.etcd.io/etcd/pkg/v3/debugutil/goroutineui"
	"go.etcd.io/etcd/pkg/v3/debugutil/pprofui"
	"go.uber.org/zap"
	"golang.org/x/net/trace"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"time"
)

// Endpoint is the entry point under which the debug tools are housed.
const Endpoint = "/debug/"

// Server serves the /debug/* family of tools.
type Server struct {
	mux *http.ServeMux
}

// NewServer sets up a debug server.
func NewServer() *Server {
	mux := http.NewServeMux()

	// Install a redirect to the UI's collection of debug tools.
	mux.HandleFunc(Endpoint, handleLanding)

	// Cribbed straight from pprof's `init()` method. See:
	// https://golang.org/src/net/http/pprof/pprof.go
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", func(w http.ResponseWriter, r *http.Request) {
		CPUProfileHandler(w, r)
	})
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// Cribbed straight from trace's `init()` method. See:
	// https://github.com/golang/net/blob/master/trace/trace.go
	mux.HandleFunc("/debug/requests", trace.Traces)
	mux.HandleFunc("/debug/events", trace.Events)

	// This registers a superset of the variables exposed through the
	// /debug/vars endpoint onto the /debug/metrics endpoint. It includes all
	// expvars registered globally and all metrics registered on the
	// DefaultRegistry.
	mux.Handle("/debug/metrics", exp.ExpHandler(metrics.DefaultRegistry))
	// Also register /debug/vars (even though /debug/metrics is better).
	mux.Handle("/debug/vars", expvar.Handler())

	ps := pprofui.NewServer(pprofui.NewMemStorage(1, 0), func(profile string, labels bool, do func()) {
		if profile != "profile" {
			do()
			return
		}

		if err := CPUProfileDo(CPUProfileOptions{WithLabels: labels}.Type(), func() error {
			var extra string
			if labels {
				extra = " (enabling profiler labels)"
			}
			log.Printf("pprofui: recording %s%s", profile, extra)
			do()
			return nil
		}); err != nil {
			// NB: we don't have good error handling here. Could be changed if we find
			// this problematic. In practice, `do()` wraps the pprof handler which will
			// return an error if there's already a profile going on just the same.
			return
		}
	})
	mux.Handle("/debug/pprof/ui/", http.StripPrefix("/debug/pprof/ui", ps))

	mux.HandleFunc("/debug/pprof/goroutineui/", func(w http.ResponseWriter, req *http.Request) {
		dump := goroutineui.NewDump(time.Now())

		_ = req.ParseForm()
		switch req.Form.Get("sort") {
		case "count":
			dump.SortCountDesc()
		case "wait":
			dump.SortWaitDesc()
		default:
		}
		_ = dump.HTML(w)
	})

	return &Server{
		mux: mux,
	}
}

// ServeHTTP serves various tools under the /debug endpoint. It restricts access
// according to the `server.remote_debugging.mode` cluster variable.
func (ds *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, _ := ds.mux.Handler(r)
	handler.ServeHTTP(w, r)
}

func handleLanding(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != Endpoint {
		http.Redirect(w, r, Endpoint, http.StatusMovedPermanently)
		return
	}

	// The explicit header is necessary or (at least Chrome) will try to
	// download a gzipped file (Content-type comes back application/x-gzip).
	w.Header().Add("Content-type", "text/html")

	fmt.Fprint(w, `
<html>
<head>
<meta charset="UTF-8">
<meta http-equiv="refresh" content="1; url=/#/debug">
<script type="text/javascript">
	window.location.href = "/#/debug"
</script>
<title>Page Redirection</title>
</head>
<body>
This page has moved.
If you are not redirected automatically, follow this <a href='/#/debug'>link</a>.
</body>
</html>
`)
}

// http://127.0.0.1/debug/pprof/ui/
// http://127.0.0.1/debug/pprof/ui/allocs
// http://127.0.0.1/debug/pprof/ui/profile
// http://127.0.0.1/debug/pprof/ui/heap
// http://127.0.0.1/debug/pprof/goroutineui/

// http://127.0.0.1/debug/pprof/ui/block
// http://127.0.0.1/debug/pprof/ui/goroutine
// http://127.0.0.1/debug/pprof/ui/mutex
// http://127.0.0.1/debug/pprof/ui/threadcreate

func StartUIPprofListener(logger *zap.Logger) {
	pprofServer := NewServer()
	listener, err := net.Listen("tcp", ":80")
	if err != nil {
		logger.Error("StartUIPprofListener start error", zap.Error(err))
		return
	}

	srvhttp := &http.Server{
		Handler: pprofServer.mux,
	}
	go func() {
		err = srvhttp.Serve(listener)
		if err != nil {
			logger.Error("srvhttp.Serve error", zap.Error(err))
			return
		}
	}()
	return
}
