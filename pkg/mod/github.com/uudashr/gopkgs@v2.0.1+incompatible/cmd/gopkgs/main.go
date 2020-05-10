package main

import (
	"bufio"
	"flag"
	"fmt"
	"html/template"
	"os"
	"runtime/pprof"
	"runtime/trace"
	"text/tabwriter"

	"github.com/uudashr/gopkgs"
)

var usageInfo = `
Use -format to custom the output using template syntax. The struct being passed to template is:
	type Pkg struct {
		Dir        string // directory containing package sources
		ImportPath string // import path of package in dir
		Name       string // package name
	}

Use -workDir={path} to speed up the package search. This will ignore any vendor package outside the package root.
`

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr)
	tw := tabwriter.NewWriter(os.Stderr, 0, 0, 4, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, usageInfo)
}

func init() {
	flag.Usage = usage
}

func main() {
	var (
		flagFormat         = flag.String("format", "{{.ImportPath}}", "custom output format")
		flagWorkDir        = flag.String("workDir", "", "importable packages only for workDir")
		flagNoVendor       = flag.Bool("no-vendor", false, "exclude vendor dependencies except under workDir (if specified)")
		flagHelp           = flag.Bool("help", false, "show this message")
		flagPerfCPUProfile *string
		flagPerfTrace      *string
	)

	envDevMode := os.Getenv("DEV_MODE")
	if envDevMode == "1" || envDevMode == "true" || envDevMode == "on" {
		flagPerfCPUProfile = flag.String("perf-cpuprofile", "", "Write the CPU profile to a file")
		flagPerfTrace = flag.String("perf-trace", "", "Write an execution trace to a file")
	}

	flag.Parse()
	if len(flag.Args()) > 0 || *flagHelp {
		flag.Usage()
		os.Exit(1)
	}

	if flagPerfCPUProfile != nil && *flagPerfCPUProfile != "" {
		pf, err := os.Create(*flagPerfCPUProfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		defer func() {
			if err = pf.Close(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}()

		if err = pprof.StartCPUProfile(pf); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		defer pprof.StopCPUProfile()
	}

	if flagPerfTrace != nil && *flagPerfTrace != "" {
		tf, err := os.Create(*flagPerfTrace)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		defer func() {
			if err = tf.Close(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}()

		if err = trace.Start(tf); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		defer trace.Stop()
	}

	tpl, err := template.New("out").Parse(*flagFormat)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	pkgs, err := gopkgs.List(gopkgs.Options{
		WorkDir:  *flagWorkDir,
		NoVendor: *flagNoVendor,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	w := bufio.NewWriter(os.Stdout)
	defer func() {
		if err := w.Flush(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()

	for _, pkg := range pkgs {
		if err := tpl.Execute(w, pkg); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(w)
	}
}
