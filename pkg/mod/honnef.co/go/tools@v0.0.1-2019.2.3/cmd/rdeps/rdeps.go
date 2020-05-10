// rdeps scans GOPATH for all reverse dependencies of a set of Go
// packages.
//
// rdeps will not sort its output, and the order of the output is
// undefined. Pipe its output through sort if you need stable output.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/build"
	"os"

	"honnef.co/go/tools/version"

	"github.com/kisielk/gotool"
	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/refactor/importgraph"
)

func main() {
	var tags buildutil.TagsFlag
	flag.Var(&tags, "tags", "List of build tags")
	stdin := flag.Bool("stdin", false, "Read packages from stdin instead of the command line")
	recursive := flag.Bool("r", false, "Print reverse dependencies recursively")
	printVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *printVersion {
		version.Print()
		os.Exit(0)
	}

	ctx := build.Default
	ctx.BuildTags = tags
	var args []string
	if *stdin {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			args = append(args, s.Text())
		}
	} else {
		args = flag.Args()
	}
	if len(args) == 0 {
		return
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	pkgs := gotool.ImportPaths(args)
	for i, pkg := range pkgs {
		bpkg, err := ctx.Import(pkg, wd, build.FindOnly)
		if err != nil {
			continue
		}
		pkgs[i] = bpkg.ImportPath
	}
	_, reverse, errors := importgraph.Build(&ctx)
	_ = errors

	seen := map[string]bool{}
	var printRDeps func(pkg string)
	printRDeps = func(pkg string) {
		for rdep := range reverse[pkg] {
			if seen[rdep] {
				continue
			}
			seen[rdep] = true
			fmt.Println(rdep)
			if *recursive {
				printRDeps(rdep)
			}
		}
	}

	for _, pkg := range pkgs {
		printRDeps(pkg)
	}
	for pkg, err := range errors {
		fmt.Fprintf(os.Stderr, "error in package %s: %s\n", pkg, err)
	}
}
