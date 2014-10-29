package main

import (
	"flag"
	"log"

	"github.com/coreos/etcd/migrate"
)

func main() {
	from := flag.String("data-dir", "", "etcd v0.4 data-dir")
	name := flag.String("name", "", "etcd node name")
	flag.Parse()

	if *from == "" {
		log.Fatal("Must provide -data-dir flag")
	}

	err := migrate.Migrate4To5(*from, *name)
	if err != nil {
		log.Fatalf("Failed migrating data-dir: %v", err)
	}
}
