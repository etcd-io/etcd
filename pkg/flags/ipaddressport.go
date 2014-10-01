package flags

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// IPAddressPort implements the flag.Value interface. The argument
// is validated as "ip:port".
type IPAddressPort struct {
	IP   string
	Port int
}

func (a *IPAddressPort) Set(arg string) error {
	arg = strings.TrimSpace(arg)

	parts := strings.SplitN(arg, ":", 2)
	if len(parts) != 2 {
		return errors.New("bad format in address specification")
	}

	if net.ParseIP(parts[0]) == nil {
		return errors.New("bad IP in address specification")
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return errors.New("bad port in address specification")
	}

	a.IP = parts[0]
	a.Port = port

	return nil
}

func (a *IPAddressPort) String() string {
	return fmt.Sprintf("%s:%d", a.IP, a.Port)
}
