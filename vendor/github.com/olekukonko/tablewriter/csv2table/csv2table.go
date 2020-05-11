package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/olekukonko/tablewriter"
)

var (
	fileName  = flag.String("f", "", "Set file with  eg. sample.csv")
	delimiter = flag.String("d", ",", "Set CSV File delimiter eg. ,|;|\t ")
	header    = flag.Bool("h", true, "Set header options eg. true|false ")
	align     = flag.String("a", "none", "Set aligmement with eg. none|left|right|center")
	pipe      = flag.Bool("p", false, "Suport for Piping from STDIN")
	border    = flag.Bool("b", true, "Enable / disable table border")
)

func main() {
	flag.Parse()
	fmt.Println()
	if *pipe || hasArg("-p") {
		process(os.Stdin)
	} else {
		if *fileName == "" {
			fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
			flag.PrintDefaults()
			fmt.Println()
			os.Exit(1)
		}
		processFile()
	}
	fmt.Println()
}

func hasArg(name string) bool {
	for _, v := range os.Args {
		if name == v {
			return true
		}
	}
	return false
}
func processFile() {
	r, err := os.Open(*fileName)
	if err != nil {
		exit(err)
	}
	defer r.Close()
	process(r)
}
func process(r io.Reader) {
	csvReader := csv.NewReader(r)
	rune, size := utf8.DecodeRuneInString(*delimiter)
	if size == 0 {
		rune = ','
	}
	csvReader.Comma = rune

	table, err := tablewriter.NewCSVReader(os.Stdout, csvReader, *header)

	if err != nil {
		exit(err)
	}

	switch *align {
	case "left":
		table.SetAlignment(tablewriter.ALIGN_LEFT)
	case "right":
		table.SetAlignment(tablewriter.ALIGN_RIGHT)
	case "center":
		table.SetAlignment(tablewriter.ALIGN_CENTER)
	}
	table.SetBorder(*border)
	table.Render()
}

func exit(err error) {
	fmt.Fprintf(os.Stderr, "#Error : %s", err)
	os.Exit(1)
}
