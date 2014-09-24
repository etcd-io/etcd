package pkg

import (
	"flag"
	"fmt"
	"log"
	"os"
)

type DeprecatedFlag struct {
	Name string
}

// IsBoolFlag is defined to allow the flag to be defined without an argument
func (df *DeprecatedFlag) IsBoolFlag() bool {
	return true
}

func (df *DeprecatedFlag) Set(s string) error {
	log.Printf("WARNING: flag \"-%s\" is no longer supported.", df.Name)
	return nil
}

func (df *DeprecatedFlag) String() string {
	return ""
}

func UsageWithIgnoredFlagsFunc(fs *flag.FlagSet, ignore []string) func() {
	iMap := make(map[string]struct{}, len(ignore))
	for _, name := range ignore {
		iMap[name] = struct{}{}
	}

	return func() {
		fs.VisitAll(func(f *flag.Flag) {
			if _, ok := iMap[f.Name]; ok {
				return
			}

			format := "  -%s=%s: %s\n"
			fmt.Fprintf(os.Stderr, format, f.Name, f.DefValue, f.Usage)
		})
	}
}
