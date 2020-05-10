package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/haya14busa/goplay"
	"github.com/skratchdot/open-golang/open"
)

var (
	help        = flag.Bool("h", false, "show help")
	run         = flag.Bool("run", true, "compile and run Go program on The Go Playground")
	share       = flag.Bool("share", true, "share code on The Go Playground")
	openBrowser = flag.Bool("openbrowser", true, "open browser automatically")
)

func main() {
	flag.Parse()

	if *help || (!*run && !*share) {
		showHelp()
		os.Exit(1)
	}

	code := os.Stdin
	if len(flag.Args()) > 0 {
		path := flag.Arg(0)
		if path != "-" { // use stdin
			file, err := os.Open(path)
			fatalIf(err)
			defer file.Close()
			code = file
		}
	}

	cli := goplay.DefaultClient

	b, err := ioutil.ReadAll(code)
	fatalIf(err)

	if string(b) == "" {
		fmt.Println("INPUT code is empty")
		fmt.Println("")
		showHelp()
		os.Exit(1)
	}

	if *run {
		if err := cli.Run(bytes.NewReader(b), os.Stdout, os.Stderr); err != nil {
			fatalIf(err)
		}
	}

	if *share {
		url, err := cli.Share(bytes.NewReader(b))
		fatalIf(err)

		fmt.Println(url)
		if *openBrowser {
			fatalIf(open.Start(url))
		}
	}
}

func fatalIf(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("SYNOPSIS: goplay [OPTION]... [FILE] (With no FILE, or when FILE is -, read standard input.")
	fmt.Println("")
	flag.PrintDefaults()
}
