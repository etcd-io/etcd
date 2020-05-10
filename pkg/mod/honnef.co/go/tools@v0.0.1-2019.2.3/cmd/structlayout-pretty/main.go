// structlayout-pretty formats the output of structlayout with ASCII
// art.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	st "honnef.co/go/tools/structlayout"
	"honnef.co/go/tools/version"
)

var (
	fVerbose bool
	fVersion bool
)

func init() {
	flag.BoolVar(&fVerbose, "v", false, "Do not compact consecutive bytes of fields")
	flag.BoolVar(&fVersion, "version", false, "Print version and exit")
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	if fVersion {
		version.Print()
		os.Exit(0)
	}

	var fields []st.Field
	if err := json.NewDecoder(os.Stdin).Decode(&fields); err != nil {
		log.Fatal(err)
	}
	if len(fields) == 0 {
		return
	}
	max := fields[len(fields)-1].End
	maxLength := len(fmt.Sprintf("%d", max))
	padding := strings.Repeat(" ", maxLength+2)
	format := fmt.Sprintf(" %%%dd ", maxLength)
	pos := int64(0)
	fmt.Println(padding + "+--------+")
	for _, field := range fields {
		name := field.Name + " " + field.Type
		if field.IsPadding {
			name = "padding"
		}
		fmt.Printf(format+"|        | <- %s (size %d, align %d)\n", pos, name, field.Size, field.Align)
		fmt.Println(padding + "+--------+")

		if fVerbose {
			for i := int64(0); i < field.Size-1; i++ {
				fmt.Printf(format+"|        |\n", pos+i+1)
				fmt.Println(padding + "+--------+")
			}
		} else {
			if field.Size > 2 {
				fmt.Println(padding + "-........-")
				fmt.Println(padding + "+--------+")
				fmt.Printf(format+"|        |\n", pos+field.Size-1)
				fmt.Println(padding + "+--------+")
			}
		}
		pos += field.Size
	}
}
