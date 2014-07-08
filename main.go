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
	rTLS, rerr := config.PeerTLSInfo().ServerConfig()

	go e.Run()

	go func() {
		l, err := net.Listen("tcp", config.Peer.BindAddr)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("raft server starts listening on", config.Peer.BindAddr)

		switch config.PeerTLSInfo().Scheme() {
		case "http":
			log.Println("raft server starts serving HTTP")

		case "https":
			if rTLS == nil {
				log.Fatal("failed to create raft tls:", rerr)
			}
			l = tls.NewListener(l, rTLS)
			log.Println("raft server starts serving HTTPS")
		default:
			log.Fatal("unsupported http scheme", config.PeerTLSInfo().Scheme())
		}

		log.Fatal(http.Serve(l, e.RaftHandler()))
	}()

	if err := http.ListenAndServe(config.BindAddr, e); err != nil {
		log.Fatal("system", err)
	}
}

func genId() int {
	r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	return r.Int()
}
