package main

import (
	"bytes"
	"fmt"
	"go/build"
	"go/importer"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"runtime/debug"
	"time"

	"github.com/mdempsky/gocode/internal/cache"
	"github.com/mdempsky/gocode/internal/gbimporter"
	"github.com/mdempsky/gocode/internal/suggest"
)

func doServer(cache bool) {
	addr := *g_addr
	if *g_sock == "unix" {
		addr = getSocketPath()
	}

	lis, err := net.Listen(*g_sock, addr)
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal)
	signal.Notify(sigs, os.Interrupt)
	go func() {
		<-sigs
		exitServer()
	}()

	if err = rpc.Register(&Server{
		cache: cache,
	}); err != nil {
		log.Fatal(err)
	}
	rpc.Accept(lis)
}

func exitServer() {
	if *g_sock == "unix" {
		_ = os.Remove(getSocketPath())
	}
	os.Exit(0)
}

type Server struct {
	cache bool
}

type AutoCompleteRequest struct {
	Filename           string
	Data               []byte
	Cursor             int
	Context            cache.PackedContext
	Source             bool
	Builtin            bool
	IgnoreCase         bool
	UnimportedPackages bool
	FallbackToSource   bool
}

type AutoCompleteReply struct {
	Candidates []suggest.Candidate
	Len        int
}

func (s *Server) AutoComplete(req *AutoCompleteRequest, res *AutoCompleteReply) error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("panic: %s\n\n", err)
			debug.PrintStack()

			res.Candidates = []suggest.Candidate{
				{Class: "PANIC", Name: "PANIC", Type: "PANIC"},
			}
		}
	}()
	if *g_debug {
		var buf bytes.Buffer
		log.Printf("Got autocompletion request for '%s'\n", req.Filename)
		log.Printf("Cursor at: %d\n", req.Cursor)
		buf.WriteString("-------------------------------------------------------\n")
		buf.Write(req.Data[:req.Cursor])
		buf.WriteString("#")
		buf.Write(req.Data[req.Cursor:])
		log.Print(buf.String())
		log.Println("-------------------------------------------------------")
	}
	now := time.Now()
	cfg := suggest.Config{
		Builtin:            req.Builtin,
		IgnoreCase:         req.IgnoreCase,
		UnimportedPackages: req.UnimportedPackages,
		Logf:               func(string, ...interface{}) {},
	}
	cfg.Logf = func(string, ...interface{}) {}
	if *g_debug {
		cfg.Logf = log.Printf
	}
	// TODO(rstambler): Figure out why this happens sometimes.
	if req.Context.GOPATH == "" || req.Context.GOROOT == "" {
		req.Context = cache.PackContext(&build.Default)
	}
	if req.Source {
		cfg.Importer = gbimporter.New(&req.Context, req.Filename, importer.For("source", nil), func(s string, args ...interface{}) {
			cfg.Logf("source: "+s, args...)
		})
	} else if s.cache {
		cache.Mu.Lock()
		defer cache.Mu.Unlock()
		cfg.Importer = cache.NewImporter(&req.Context, req.Filename, req.FallbackToSource, func(s string, args ...interface{}) {
			cfg.Logf("cache: "+s, args...)
		})
	} else {
		cfg.Importer = gbimporter.New(&req.Context, req.Filename, importer.Default(), func(s string, args ...interface{}) {
			cfg.Logf("gbimporter: "+s, args...)
		})
	}

	candidates, d := cfg.Suggest(req.Filename, req.Data, req.Cursor)
	elapsed := time.Since(now)
	if *g_debug {
		log.Printf("Elapsed duration: %v\n", elapsed)
		log.Printf("Offset: %d\n", res.Len)
		log.Printf("Number of candidates found: %d\n", len(candidates))
		log.Printf("Candidates are:\n")
		for _, c := range candidates {
			log.Printf("  %s\n", c.String())
		}
		log.Println("=======================================================")
	}
	res.Candidates, res.Len = candidates, d
	return nil
}

type ExitRequest struct{}
type ExitReply struct{}

func (s *Server) Exit(req *ExitRequest, res *ExitReply) error {
	go func() {
		time.Sleep(time.Second)
		exitServer()
	}()
	return nil
}
