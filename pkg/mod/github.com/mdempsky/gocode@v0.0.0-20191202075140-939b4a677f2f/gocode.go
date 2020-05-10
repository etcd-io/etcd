package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var (
	g_is_server           = flag.Bool("s", false, "run a server instead of a client")
	g_cache               = flag.Bool("cache", false, "use the cache importer")
	g_format              = flag.String("f", "nice", "output format (vim | emacs | sexp | nice | csv | json)")
	g_input               = flag.String("in", "", "use this file instead of stdin input")
	g_sock                = flag.String("sock", defaultSocketType, "socket type (unix | tcp | none)")
	g_addr                = flag.String("addr", "127.0.0.1:37373", "address for tcp socket")
	g_debug               = flag.Bool("debug", false, "enable server-side debug mode")
	g_source              = flag.Bool("source", false, "use source importer")
	g_builtin             = flag.Bool("builtin", false, "propose completions for built-in functions and types")
	g_ignore_case         = flag.Bool("ignore-case", false, "do case-insensitive matching")
	g_unimported_packages = flag.Bool("unimported-packages", false, "propose completions for standard library packages not explicitly imported")
	g_fallback_to_source  = flag.Bool("fallback-to-source", false, "if importing a package fails, fallback to the source importer")
)

func getSocketPath() string {
	user := os.Getenv("USER")
	if user == "" {
		user = "all"
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("gocode-daemon.%s", user))
}

func usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [-s] [-f=<format>] [-in=<path>] [-sock=<type>] [-addr=<addr>]\n"+
			"       <command> [<args>]\n\n",
		os.Args[0])
	fmt.Fprintf(os.Stderr,
		"Flags:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr,
		"\nCommands:\n"+
			"  autocomplete [<path>] <offset>     main autocompletion command\n"+
			"  exit                               terminate the gocode daemon\n")
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *g_is_server {
		doServer(*g_cache)
	} else {
		doClient()
	}
}
