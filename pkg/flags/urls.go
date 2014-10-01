package flags

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// URLs implements the flag.Value interface to allow users to define multiple
// URLs on the command-line
type URLs []url.URL

// Set parses a command line set of URLs formatted like:
// http://127.0.0.1:7001,http://10.1.1.2:80
func (us *URLs) Set(s string) error {
	strs := strings.Split(s, ",")
	all := make([]url.URL, len(strs))
	if len(all) == 0 {
		return errors.New("no valid URLs given")
	}
	for i, in := range strs {
		in = strings.TrimSpace(in)
		u, err := url.Parse(in)
		if err != nil {
			return err
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("URL scheme must be http or https: %s", s)
		}
		if u.Path != "" {
			return fmt.Errorf("URL must not contain a path: %s", s)
		}
		all[i] = *u
	}
	*us = all
	return nil
}

func (us *URLs) String() string {
	all := make([]string, len(*us))
	for i, u := range *us {
		all[i] = u.String()
	}
	return strings.Join(all, ",")
}
