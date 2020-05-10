package pkg

import (
	"bytes"
	"net"
)

func fn() {
	type T []byte
	var i1, i2 net.IP
	var b1, b2 []byte
	var t1, t2 T

	bytes.Equal(i1, i2) // want `use net\.IP\.Equal to compare net\.IPs, not bytes\.Equal`
	bytes.Equal(b1, b2)
	bytes.Equal(t1, t2)

	bytes.Equal(i1, b1)
	bytes.Equal(b1, i1)
}
