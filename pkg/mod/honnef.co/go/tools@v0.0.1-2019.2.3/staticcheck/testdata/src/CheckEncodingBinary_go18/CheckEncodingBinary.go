package pkg

import (
	"encoding/binary"
	"io/ioutil"
)

func fn() {
	var x bool
	binary.Write(ioutil.Discard, binary.LittleEndian, x)
}
