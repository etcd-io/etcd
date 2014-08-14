package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/etcd"
	ehttp "github.com/coreos/etcd/http"
)

var version = "etcd <unknown>"

func main() {
	var config = config.New()
	if err := config.Load(os.Args[1:]); err != nil {
		fmt.Println(etcd.Usage() + "\n")
		fmt.Println(err.Error(), "\n")
		os.Exit(1)
	} else if config.ShowVersion {
		fmt.Println(version)
		os.Exit(0)
	} else if config.ShowHelp {
		os.Exit(0)
	}

	e, err := etcd.New(config)
	if err != nil {
		log.Fatal("etcd:", err)
	}
	go e.Run()

	corsInfo, err := ehttp.NewCORSInfo(config.CorsOrigins)
	if err != nil {
		log.Fatal("cors:", err)
	}

	readTimeout := time.Duration(config.HTTPReadTimeout) * time.Second
	writeTimeout := time.Duration(config.HTTPWriteTimeout) * time.Second
	go func() {
		serve("raft", config.Peer.BindAddr, config.PeerTLSInfo(), corsInfo, e.RaftHandler(), readTimeout, writeTimeout)
	}()
	serve("etcd", config.BindAddr, config.EtcdTLSInfo(), corsInfo, e, readTimeout, writeTimeout)
}

func serve(who string, addr string, tinfo *config.TLSInfo, cinfo *ehttp.CORSInfo, handler http.Handler, readTimeout, writeTimeout time.Duration) {
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

	h := &ehttp.CORSHandler{handler, cinfo}
	s := &http.Server{Handler: h, ReadTimeout: readTimeout, WriteTimeout: writeTimeout}
	log.Fatal(s.Serve(l))
}
