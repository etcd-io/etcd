package pkg

import (
	"encoding/binary"
	"io/ioutil"
	"log"
)

func fn() {
	var x bool
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x)) // want `cannot be used with binary\.Write`
}
