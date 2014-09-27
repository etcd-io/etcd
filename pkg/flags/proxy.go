package flags

import (
	"errors"
)

const (
	ProxyValueOff      = "off"
	ProxyValueReadonly = "readonly"
	ProxyValueOn       = "on"
)

var (
	ProxyValues = []string{
		ProxyValueOff,
		ProxyValueReadonly,
		ProxyValueOn,
	}
)

// ProxyFlag implements the flag.Value interface.
type Proxy string

// Set verifies the argument to be a valid member of proxyFlagValues
// before setting the underlying flag value.
func (pf *Proxy) Set(s string) error {
	for _, v := range ProxyValues {
		if s == v {
			*pf = Proxy(s)
			return nil
		}
	}

	return errors.New("invalid value")
}

func (pf *Proxy) String() string {
	return string(*pf)
}
