package main

import (
	"fmt"
	"go.etcd.io/etcd/v3/etcdmain"
)

func main() {
	fmt.Println("Hello World")
	etcdmain.Main()
}
