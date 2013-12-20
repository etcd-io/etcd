package main

import (
	"flag"
	"log"
	"strconv"

	"github.com/coreos/go-etcd/etcd"
)

func doWrite(client *etcd.Client, key string, c chan int) {
	client.Set(key, key, 0)
	c <- 1
}

func write(client *etcd.Client, requests int, end chan int) {
	c := make(chan int)

	for i := 0; i < requests; i++ {
		key := strconv.Itoa(i)
		go doWrite(client, key, c)
		<-c
	}
	end <- 1
}

func watch(client *etcd.Client, key string) {
	receiver := make(chan *etcd.Response)
	go client.Watch(key, 0, true, receiver, nil)

	log.Printf("watching: %s", key)

	received := 0
	for {
		<-receiver
		received++
	}
}

func main() {
	rWrites := flag.Int("write-requests", 50000, "number of writes")
	cWrites := flag.Int("concurrent-writes", 500, "number of concurrent writes")

	watches := flag.Int("watches", 500, "number of writes")

	flag.Parse()

	client := etcd.NewClient(nil)

	for i := 0; i < *watches; i++ {
		key := strconv.Itoa(i)
		go watch(client, key)
	}

	wChan := make(chan int, *cWrites)
	for i := 0; i < *cWrites; i++ {
		go write(client, (*rWrites / *cWrites), wChan)
	}

	for i := 0; i < *cWrites; i++ {
		<-wChan
		log.Printf("Completed %d writes", (*rWrites / *cWrites))
	}
}
