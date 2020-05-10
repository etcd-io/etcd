package main

import (
	"flag"
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"runtime/debug"

	"github.com/mdempsky/gocode/internal/cache"
	"github.com/mdempsky/gocode/internal/suggest"
)

func doClient() {
	// Client is a short-lived program.
	// Disable GC to make it faster
	debug.SetGCPercent(-1)

	if *g_debug {
		start := time.Now()
		defer func() {
			elapsed := time.Since(start)
			log.Printf("Elapsed duration: %v\n", elapsed)
		}()
	}

	var command string
	if flag.NArg() > 0 {
		command = flag.Arg(0)
		switch command {
		case "autocomplete", "exit":
			// these are valid commands
		case "close":
			// "close" is an alias for "exit"
			command = "exit"
		default:
			fmt.Printf("gocode: unknown subcommand: %q\nRun 'gocode -help' for usage.\n", command)
			os.Exit(2)
		}
	}

	// client
	var client *rpc.Client
	if *g_sock != "none" {
		addr := *g_addr
		if *g_sock == "unix" {
			addr = getSocketPath()
		}

		var err error
		client, err = rpc.Dial(*g_sock, addr)
		if err != nil {
			if command == "exit" {
				log.Fatal(err)
			}

			if *g_sock == "unix" {
				_ = os.Remove(addr)
			}
			err = tryStartServer()
			if err != nil {
				log.Fatalf("Failed to start server: %s\n", err)
			}
			client, err = tryToConnect(*g_sock, addr)
			if err != nil {
				log.Fatalf("Failed to connect to %q: %s\n", addr, err)
			}
		}
		defer client.Close()
	}

	switch command {
	case "autocomplete":
		cmdAutoComplete(client)
	case "exit":
		cmdExit(client)
	}
}

func tryStartServer() error {
	path := get_executable_filename()
	args := []string{os.Args[0], "-s", "-sock", *g_sock, "-addr", *g_addr}
	if *g_cache {
		args = append(args, "-cache")
	}
	cwd, _ := os.Getwd()

	var err error
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	stdout, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	stderr, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}

	procattr := os.ProcAttr{Dir: cwd, Env: os.Environ(), Files: []*os.File{stdin, stdout, stderr}}
	p, err := os.StartProcess(path, args, &procattr)
	if err != nil {
		return err
	}

	return p.Release()
}

func tryToConnect(network, address string) (*rpc.Client, error) {
	start := time.Now()
	for {
		client, err := rpc.Dial(network, address)
		if err != nil && time.Since(start) < time.Second {
			continue
		}
		return client, err
	}
}

func cmdAutoComplete(c *rpc.Client) {
	var req AutoCompleteRequest
	req.Filename, req.Data, req.Cursor = prepareFilenameDataCursor()
	req.Context = cache.PackContext(&build.Default)
	req.Source = *g_source
	req.Builtin = *g_builtin
	req.IgnoreCase = *g_ignore_case
	req.UnimportedPackages = *g_unimported_packages
	req.FallbackToSource = *g_fallback_to_source

	var res AutoCompleteReply
	var err error
	if c == nil {
		s := Server{}
		err = s.AutoComplete(&req, &res)
	} else {
		err = c.Call("Server.AutoComplete", &req, &res)
	}
	if err != nil {
		log.Fatal(err)
	}

	fmt := suggest.Formatters[*g_format]
	if fmt == nil {
		fmt = suggest.NiceFormat
	}
	fmt(os.Stdout, res.Candidates, res.Len)
}

func cmdExit(c *rpc.Client) {
	if c == nil {
		return
	}
	var req ExitRequest
	var res ExitReply
	if err := c.Call("Server.Exit", &req, &res); err != nil {
		log.Fatal(err)
	}
}

func prepareFilenameDataCursor() (string, []byte, int) {
	var file []byte
	var err error

	if *g_input != "" {
		file, err = ioutil.ReadFile(*g_input)
	} else {
		file, err = ioutil.ReadAll(os.Stdin)
	}

	if err != nil {
		log.Fatal(err)
	}

	filename := *g_input
	offset := ""
	switch flag.NArg() {
	case 2:
		offset = flag.Arg(1)
	case 3:
		filename = flag.Arg(1) // Override default filename
		offset = flag.Arg(2)
	}

	if filename != "" {
		filename, _ = filepath.Abs(filename)
	}

	cursor := -1
	if offset != "" {
		if offset[0] == 'c' || offset[0] == 'C' {
			cursor, _ = strconv.Atoi(offset[1:])
			cursor = runeToByteOffset(file, cursor)
		} else {
			cursor, _ = strconv.Atoi(offset)
		}
	}

	return filename, file, cursor
}
