package flags

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// Addrs implements the flag.Value interface to allow users to define multiple
// listen addresses on the command-line
type Addrs []string

// Set parses a command line set of listen addresses, formatted like:
// 127.0.0.1:7001,10.1.1.2:80
func (as *Addrs) Set(s string) error {
	parsed := make([]string, 0)
	for _, in := range strings.Split(s, ",") {
		a := strings.TrimSpace(in)
		if err := validateAddr(a); err != nil {
			return err
		}
		parsed = append(parsed, a)
	}
	if len(parsed) == 0 {
		return errors.New("no valid addresses given!")
	}
	*as = parsed
	return nil
}

func (as *Addrs) String() string {
	return strings.Join(*as, ",")
}

// validateAddr ensures that the provided string is a valid address. Valid
// addresses are of the form IP:port.
// Returns an error if the address is invalid, else nil.
func validateAddr(s string) error {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return errors.New("bad format in address specification")
	}
	if net.ParseIP(parts[0]) == nil {
		return errors.New("bad IP in address specification")
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return errors.New("bad port in address specification")
	}
	return nil
}
