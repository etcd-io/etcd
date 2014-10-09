package main

import (
	"flag"
	"log"

	"github.com/coreos/etcd/migrate"
)

func main() {
	from := flag.String("data-dir", "", "etcd v0.4 data-dir")
	flag.Parse()

	if *from == "" {
		log.Fatal("Must provide -from flag")
	}

	err := migrate.Migrate4To5(*from)
	if err != nil {
		log.Fatalf("Failed migrating data-dir: %v", err)
	}
}
