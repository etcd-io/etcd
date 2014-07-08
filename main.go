package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/etcd"
)

func main() {
	var config = config.New()
	if err := config.Load(os.Args[1:]); err != nil {
		fmt.Println(err.Error(), "\n")
		os.Exit(1)
	} else if config.ShowVersion {
		fmt.Println("0.5")
		os.Exit(0)
	} else if config.ShowHelp {
		os.Exit(0)
	}

	e := etcd.New(config, genId())
	go e.Run()

	go func() {
		serve("raft", config.Peer.BindAddr, config.PeerTLSInfo(), e.RaftHandler())
	}()
	serve("etcd", config.BindAddr, config.EtcdTLSInfo(), e)
}

func genId() int {
	r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	return r.Int()
}

func serve(who string, addr string, info *config.TLSInfo, handler http.Handler) {
	t, terr := info.ServerConfig()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%v server starts listening on %v\n", who, addr)

	switch info.Scheme() {
	case "http":
		log.Printf("%v server starts serving HTTP\n", who)

	case "https":
		if t == nil {
			log.Fatalf("failed to create %v tls: %v\n", who, terr)
		}
		l = tls.NewListener(l, t)
		log.Printf("%v server starts serving HTTPS\n", who)
	default:
		log.Fatal("unsupported http scheme", info.Scheme())
	}

	log.Fatal(http.Serve(l, handler))
}
