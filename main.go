package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/etcd/etcd"
)

var (
	laddr   = flag.String("l", ":8000", "The port to listen on")
	paddr   = flag.String("p", "127.0.0.1:8000", "The public address to be adversited")
	cluster = flag.String("c", "", "The cluster to join")
)

func main() {
	flag.Parse()

	p, err := sanitizeURL(*paddr)
	if err != nil {
		log.Fatal(err)
	}

	var e *etcd.Server

	if len(*cluster) == 0 {
		e = etcd.New(1, p, nil)
		go e.Bootstrap()
	} else {
		addrs := strings.Split(*cluster, ",")
		cStr := addrs[0]
		c, err := sanitizeURL(cStr)
		if err != nil {
			log.Fatal(err)
		}
		e = etcd.New(len(addrs), p, []string{c})
		go e.Join()
	}

	if err := http.ListenAndServe(*laddr, e); err != nil {
		log.Fatal("system", err)
	}
}

func sanitizeURL(ustr string) (string, error) {
	u, err := url.Parse(ustr)
	if err != nil {
		return "", err
	}

	if u.Scheme == "" {
		u.Scheme = "http"
	}
	return u.String(), nil
}
