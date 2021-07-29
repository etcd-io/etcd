package main

import (
	"os"

	"go.etcd.io/etcd/server/v3/etcdmain"
)

func main() {
	etcdmain.Main(os.Args)
}
