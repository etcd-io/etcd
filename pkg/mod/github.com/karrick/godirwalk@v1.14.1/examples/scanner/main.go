package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karrick/godirwalk"
)

func main() {
	dirname := "."
	if flag.NArg() > 0 {
		dirname = flag.Arg(0)
	}

	scanner, err := godirwalk.NewScanner(dirname)
	if err != nil {
		fatal("cannot scan directory: %s", err)
	}

	for scanner.Scan() {
		dirent, err := scanner.Dirent()
		if err != nil {
			warning("cannot get dirent: %s", err)
			continue
		}
		name := dirent.Name()
		if name == "break" {
			break
		}
		if name == "continue" {
			continue
		}
		fmt.Printf("%v %v\n", dirent.ModeType(), name)
	}
	if err := scanner.Err(); err != nil {
		fatal("cannot scan directory: %s", err)
	}
}

var (
	optQuiet    = flag.Bool("quiet", false, "Elide printing of non-critical error messages.")
	programName string
)

func init() {
	var err error
	if programName, err = os.Executable(); err != nil {
		programName = os.Args[0]
	}
	programName = filepath.Base(programName)
	flag.Parse()
}

func stderr(f string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, programName+": "+fmt.Sprintf(f, args...)+"\n")
}

func fatal(f string, args ...interface{}) {
	stderr(f, args...)
	os.Exit(1)
}

func warning(f string, args ...interface{}) {
	if !*optQuiet {
		stderr(f, args...)
	}
}
