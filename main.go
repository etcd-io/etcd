package main

import (
	"fmt"
	"log"
	"math/rand"
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

	if err := http.ListenAndServe(config.BindAddr, e); err != nil {
		log.Fatal("system", err)
	}
}

func genId() int {
	r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	return r.Int()
}
