package pkg

import (
	"encoding/binary"
	"io/ioutil"
	"log"
)

func fn() {
	type T1 struct {
		A int32
	}
	type T2 struct {
		A int32
		B int
	}
	type T3 struct {
		A []int32
	}
	type T4 struct {
		A *int32
	}
	type T5 struct {
		A int32
	}
	type T6 []byte

	var x1 int
	var x2 int32
	var x3 []int
	var x4 []int32
	var x5 [1]int
	var x6 [1]int32
	var x7 T1
	var x8 T2
	var x9 T3
	var x10 T4
	var x11 = &T5{}
	var x13 []byte
	var x14 *[]byte
	var x15 T6
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x1)) // want `cannot be used with binary\.Write`
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x2))
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x3)) // want `cannot be used with binary\.Write`
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x4))
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x5)) // want `cannot be used with binary\.Write`
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x6))
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x7))
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x8))  // want `cannot be used with binary\.Write`
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x9))  // want `cannot be used with binary\.Write`
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x10)) // want `cannot be used with binary\.Write`
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x11))
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, &x13))
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, &x14)) // want `cannot be used with binary\.Write`
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, x15))
	log.Println(binary.Write(ioutil.Discard, binary.LittleEndian, &x15))
}
