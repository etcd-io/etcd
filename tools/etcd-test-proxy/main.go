// Copyright 2018 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// etcd-test-proxy is a proxy layer that simulates various network conditions.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/etcd/pkg/transport"

	"google.golang.org/grpc/grpclog"
)

var from string
var to string
var httpPort int
var verbose bool

func main() {
	// TODO: support TLS
	flag.StringVar(&from, "from", "localhost:23790", "Address URL to proxy from.")
	flag.StringVar(&to, "to", "localhost:2379", "Address URL to forward.")
	flag.IntVar(&httpPort, "http-port", 2378, "Port to serve etcd-test-proxy API.")
	flag.BoolVar(&verbose, "verbose", false, "'true' to run proxy in verbose mode.")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %q:\n", os.Args[0])
		fmt.Fprintln(os.Stderr, `
etcd-test-proxy simulates various network conditions for etcd testing purposes.
See README.md for more examples.

Example:

# build etcd
$ ./build
$ ./bin/etcd

# build etcd-test-proxy
$ make build-etcd-test-proxy -f ./hack/scripts-dev/Makefile

# to test etcd with proxy layer
$ ./bin/etcd-test-proxy --help
$ ./bin/etcd-test-proxy --from localhost:23790 --to localhost:2379 --http-port 2378 --verbose

$ ETCDCTL_API=3 ./bin/etcdctl --endpoints localhost:2379 put foo bar
$ ETCDCTL_API=3 ./bin/etcdctl --endpoints localhost:23790 put foo bar`)
		flag.PrintDefaults()
	}

	flag.Parse()

	cfg := transport.ProxyConfig{
		From: url.URL{Scheme: "tcp", Host: from},
		To:   url.URL{Scheme: "tcp", Host: to},
	}
	if verbose {
		cfg.Logger = grpclog.NewLoggerV2WithVerbosity(os.Stderr, os.Stderr, os.Stderr, 5)
	}
	p := transport.NewProxy(cfg)
	<-p.Ready()
	defer p.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(fmt.Sprintf("proxying [%s -> %s]\n", p.From(), p.To())))
	})
	mux.HandleFunc("/delay-tx", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			w.Write([]byte(fmt.Sprintf("current send latency %v\n", p.LatencyTx())))
		case http.MethodPut, http.MethodPost:
			if err := req.ParseForm(); err != nil {
				w.Write([]byte(fmt.Sprintf("wrong form %q\n", err.Error())))
				return
			}
			lat, err := time.ParseDuration(req.PostForm.Get("latency"))
			if err != nil {
				w.Write([]byte(fmt.Sprintf("wrong latency form %q\n", err.Error())))
				return
			}
			rv, err := time.ParseDuration(req.PostForm.Get("random-variable"))
			if err != nil {
				w.Write([]byte(fmt.Sprintf("wrong random-variable form %q\n", err.Error())))
				return
			}
			p.DelayTx(lat, rv)
			w.Write([]byte(fmt.Sprintf("added send latency %v±%v (current latency %v)\n", lat, rv, p.LatencyTx())))
		case http.MethodDelete:
			lat := p.LatencyTx()
			p.UndelayTx()
			w.Write([]byte(fmt.Sprintf("removed latency %v\n", lat)))
		default:
			w.Write([]byte(fmt.Sprintf("unsupported method %q\n", req.Method)))
		}
	})
	mux.HandleFunc("/delay-rx", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			w.Write([]byte(fmt.Sprintf("current receive latency %v\n", p.LatencyRx())))
		case http.MethodPut, http.MethodPost:
			if err := req.ParseForm(); err != nil {
				w.Write([]byte(fmt.Sprintf("wrong form %q\n", err.Error())))
				return
			}
			lat, err := time.ParseDuration(req.PostForm.Get("latency"))
			if err != nil {
				w.Write([]byte(fmt.Sprintf("wrong latency form %q\n", err.Error())))
				return
			}
			rv, err := time.ParseDuration(req.PostForm.Get("random-variable"))
			if err != nil {
				w.Write([]byte(fmt.Sprintf("wrong random-variable form %q\n", err.Error())))
				return
			}
			p.DelayRx(lat, rv)
			w.Write([]byte(fmt.Sprintf("added receive latency %v±%v (current latency %v)\n", lat, rv, p.LatencyRx())))
		case http.MethodDelete:
			lat := p.LatencyRx()
			p.UndelayRx()
			w.Write([]byte(fmt.Sprintf("removed latency %v\n", lat)))
		default:
			w.Write([]byte(fmt.Sprintf("unsupported method %q\n", req.Method)))
		}
	})
	mux.HandleFunc("/pause-tx", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut, http.MethodPost:
			p.PauseTx()
			w.Write([]byte(fmt.Sprintf("paused forwarding [%s -> %s]\n", p.From(), p.To())))
		case http.MethodDelete:
			p.UnpauseTx()
			w.Write([]byte(fmt.Sprintf("unpaused forwarding [%s -> %s]\n", p.From(), p.To())))
		default:
			w.Write([]byte(fmt.Sprintf("unsupported method %q\n", req.Method)))
		}
	})
	mux.HandleFunc("/pause-rx", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut, http.MethodPost:
			p.PauseRx()
			w.Write([]byte(fmt.Sprintf("paused forwarding [%s <- %s]\n", p.From(), p.To())))
		case http.MethodDelete:
			p.UnpauseRx()
			w.Write([]byte(fmt.Sprintf("unpaused forwarding [%s <- %s]\n", p.From(), p.To())))
		default:
			w.Write([]byte(fmt.Sprintf("unsupported method %q\n", req.Method)))
		}
	})
	mux.HandleFunc("/blackhole-tx", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut, http.MethodPost:
			p.BlackholeTx()
			w.Write([]byte(fmt.Sprintf("blackholed; dropping packets [%s -> %s]\n", p.From(), p.To())))
		case http.MethodDelete:
			p.UnblackholeTx()
			w.Write([]byte(fmt.Sprintf("unblackholed; restart forwarding [%s -> %s]\n", p.From(), p.To())))
		default:
			w.Write([]byte(fmt.Sprintf("unsupported method %q\n", req.Method)))
		}
	})
	mux.HandleFunc("/blackhole-rx", func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPut, http.MethodPost:
			p.BlackholeRx()
			w.Write([]byte(fmt.Sprintf("blackholed; dropping packets [%s <- %s]\n", p.From(), p.To())))
		case http.MethodDelete:
			p.UnblackholeRx()
			w.Write([]byte(fmt.Sprintf("unblackholed; restart forwarding [%s <- %s]\n", p.From(), p.To())))
		default:
			w.Write([]byte(fmt.Sprintf("unsupported method %q\n", req.Method)))
		}
	})
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: mux,
	}
	defer srv.Close()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	go func() {
		s := <-sig
		fmt.Printf("\n\nreceived signal %q, shutting down HTTP server\n\n", s)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := srv.Shutdown(ctx)
		cancel()
		fmt.Printf("gracefully stopped HTTP server with %v\n\n", err)
		os.Exit(0)
	}()

	fmt.Printf("\nserving HTTP server http://localhost:%d\n\n", httpPort)
	err := srv.ListenAndServe()
	fmt.Printf("HTTP server exit with error %v\n", err)
}
