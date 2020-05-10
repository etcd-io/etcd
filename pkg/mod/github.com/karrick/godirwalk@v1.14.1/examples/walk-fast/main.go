/*
 * walk-fast
 *
 * Walks a file system hierarchy using this library.
 */
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/karrick/godirwalk"
)

func main() {
	optVerbose := flag.Bool("verbose", false, "Print file system entries.")
	flag.Parse()

	dirname := "."
	if flag.NArg() > 0 {
		dirname = flag.Arg(0)
	}

	err := godirwalk.Walk(dirname, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			if *optVerbose {
				fmt.Printf("%s %s\n", de.ModeType(), osPathname)
			}
			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			if *optVerbose {
				fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			}

			// For the purposes of this example, a simple SkipNode will suffice,
			// although in reality perhaps additional logic might be called for.
			return godirwalk.SkipNode
		},
		Unsorted: true, // set true for faster yet non-deterministic enumeration (see godoc)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
