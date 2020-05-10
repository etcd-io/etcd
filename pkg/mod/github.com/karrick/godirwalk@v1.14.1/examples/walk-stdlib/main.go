/*
 * walk-fast
 *
 * Walks a file system hierarchy using the standard library.
 */
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	optVerbose := flag.Bool("verbose", false, "Print file system entries.")
	flag.Parse()

	dirname := "."
	if flag.NArg() > 0 {
		dirname = flag.Arg(0)
	}

	err := filepath.Walk(dirname, func(osPathname string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if *optVerbose {
			fmt.Printf("%s %s\n", info.Mode(), osPathname)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
