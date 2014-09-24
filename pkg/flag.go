package pkg

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
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

// SetFlagsFromEnv parses all registered flags in the given flagset,
// and if they are not already set it attempts to set their values from
// environment variables. Environment variables take the name of the flag but
// are UPPERCASE, have the prefix "ETCD_", and any dashes are replaced by
// underscores - for example: some-flag => ETCD_SOME_FLAG
func SetFlagsFromEnv(fs *flag.FlagSet) {
	alreadySet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		alreadySet[f.Name] = true
	})
	fs.VisitAll(func(f *flag.Flag) {
		if !alreadySet[f.Name] {
			key := "ETCD_" + strings.ToUpper(strings.Replace(f.Name, "-", "_", -1))
			val := os.Getenv(key)
			if val != "" {
				fs.Set(f.Name, val)
			}
		}
	})
}
