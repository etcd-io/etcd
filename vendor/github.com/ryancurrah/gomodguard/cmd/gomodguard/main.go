package main

import (
	"os"

	"github.com/ryancurrah/gomodguard/internal/cli"
)

func main() {
	os.Exit(cli.Run())
}
