package main

import (
	"flag"
	"log"
	"strconv"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

var (
	// output debug to log
	verbose *bool
)

func write(endpoint string, requests int, end chan int) {
	client := etcd.NewClient([]string{endpoint})

	for i := 0; i < requests; i++ {
		key := strconv.Itoa(i)
		_, err := client.Set(key, key, 0)
		if err != nil {
			println(err.Error())
		}
	}
	end <- 1
}

func watch(endpoint string, key string) {
	client := etcd.NewClient([]string{endpoint})

	receiver := make(chan *etcd.Response)
	go client.Watch(key, 0, true, receiver, nil)

	if *verbose {
		log.Printf("watching: %s", key)
	}

	received := 0
	for {
		<-receiver
		received++
	}
}

func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:4001", "etcd HTTP endpoint")

	rWrites := flag.Int("write-requests", 50000, "number of writes")
	cWrites := flag.Int("concurrent-writes", 500, "number of concurrent writes")
	watches := flag.Int("watches", 500, "number of concurrent watches")
	verbose = flag.Bool("verbose", false, "output debug info")

	flag.Parse()

	log.Printf("Benchmarking %v", *endpoint)
	log.Printf("%v writes with %v concurrent writers and %v watches", *rWrites, *cWrites, *watches)

	t := time.Now()
	for i := 0; i < *watches; i++ {
		key := strconv.Itoa(i)
		go watch(*endpoint, key)
	}

	wChan := make(chan int, *cWrites)
	for i := 0; i < *cWrites; i++ {
		go write(*endpoint, (*rWrites / *cWrites), wChan)
	}

	for i := 0; i < *cWrites; i++ {
		<-wChan
		if *verbose {
			log.Printf("Completed %d writes", (*rWrites / *cWrites))
		}
	}

	log.Printf("Took %v", time.Now().Sub(t))
}
