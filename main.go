package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/coreos/etcd/conf"
	"github.com/coreos/etcd/etcd"
)

func main() {
	var cfg = conf.New()
	if err := cfg.Load(os.Args[1:]); err != nil {
		fmt.Println(etcd.Usage() + "\n")
		fmt.Println(err.Error(), "\n")
		os.Exit(1)
	} else if cfg.ShowVersion {
		fmt.Println("0.5")
		os.Exit(0)
	} else if cfg.ShowHelp {
		os.Exit(0)
	}

	e, err := etcd.New(cfg)
	if err != nil {
		log.Fatal("etcd:", err)
	}
	go e.Run()

	corsInfo, err := newCORSInfo(cfg.CorsOrigins)
	if err != nil {
		log.Fatal("cors:", err)
	}

	readTimeout := time.Duration(cfg.HTTPReadTimeout) * time.Second
	writeTimeout := time.Duration(cfg.HTTPWriteTimeout) * time.Second
	go func() {
		serve("raft", cfg.Peer.BindAddr, cfg.PeerTLSInfo(), corsInfo, e.RaftHandler(), readTimeout, writeTimeout)
	}()
	serve("etcd", cfg.BindAddr, cfg.EtcdTLSInfo(), corsInfo, e, readTimeout, writeTimeout)
}

func serve(who string, addr string, tinfo *conf.TLSInfo, cinfo *CORSInfo, handler http.Handler, readTimeout, writeTimeout time.Duration) {
	t, terr := tinfo.ServerConfig()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%v server starts listening on %v\n", who, addr)

	switch tinfo.Scheme() {
	case "http":
		log.Printf("%v server starts serving HTTP\n", who)

	case "https":
		if t == nil {
			log.Fatalf("failed to create %v tls: %v\n", who, terr)
		}
		l = tls.NewListener(l, t)
		log.Printf("%v server starts serving HTTPS\n", who)
	default:
		log.Fatal("unsupported http scheme", tinfo.Scheme())
	}

	h := &CORSHandler{handler, cinfo}
	s := &http.Server{Handler: h, ReadTimeout: readTimeout, WriteTimeout: writeTimeout}
	log.Fatal(s.Serve(l))
}
