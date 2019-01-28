// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package encrypt

type randStruct struct {
	seed1       uint32
	seed2       uint32
	maxValue    uint32
	maxValueDbl float64
}

// randomInit random generation structure initialization
func (rs *randStruct) randomInit(password []byte, length int) {
	// Generate binary hash from raw text string
	var nr, add, nr2, tmp uint32
	nr = 1345345333
	add = 7
	nr2 = 0x12345671

	for i := 0; i < length; i++ {
		pswChar := password[i]
		if pswChar == ' ' || pswChar == '\t' {
			continue
		}
		tmp = uint32(pswChar)
		nr ^= (((nr & 63) + add) * tmp) + (nr << 8)
		nr2 += (nr2 << 8) ^ nr
		add += tmp
	}

	seed1 := nr & ((uint32(1) << 31) - uint32(1))
	seed2 := nr2 & ((uint32(1) << 31) - uint32(1))

	//  New (MySQL 3.21+) random generation structure initialization
	rs.maxValue = 0x3FFFFFFF
	rs.maxValueDbl = float64(rs.maxValue)
	rs.seed1 = seed1 % rs.maxValue
	rs.seed2 = seed2 % rs.maxValue
}

func (rs *randStruct) myRand() float64 {
	rs.seed1 = (rs.seed1*3 + rs.seed2) % rs.maxValue
	rs.seed2 = (rs.seed1 + rs.seed2 + 33) % rs.maxValue

	return ((float64(rs.seed1)) / rs.maxValueDbl)
}

// sqlCrypt use to store initialization results
type sqlCrypt struct {
	rand    randStruct
	orgRand randStruct

	decodeBuff [256]byte
	encodeBuff [256]byte
	shift      uint32
}

func (sc *sqlCrypt) init(password []byte, length int) {
	sc.rand.randomInit(password, length)

	for i := 0; i <= 255; i++ {
		sc.decodeBuff[i] = byte(i)
	}

	for i := 0; i <= 255; i++ {
		idx := uint32(sc.rand.myRand() * 255.0)
		a := sc.decodeBuff[idx]
		sc.decodeBuff[idx] = sc.decodeBuff[i]
		sc.decodeBuff[i] = a
	}

	for i := 0; i <= 255; i++ {
		sc.encodeBuff[sc.decodeBuff[i]] = byte(i)
	}

	sc.orgRand = sc.rand
	sc.shift = 0
}

func (sc *sqlCrypt) reinit() {
	sc.shift = 0
	sc.rand = sc.orgRand
}

func (sc *sqlCrypt) encode(str []byte, length int) {
	for i := 0; i < length; i++ {
		sc.shift ^= uint32(sc.rand.myRand() * 255.0)
		idx := uint32(str[i])
		str[i] = sc.encodeBuff[idx] ^ byte(sc.shift)
		sc.shift ^= idx
	}
}

func (sc *sqlCrypt) decode(str []byte, length int) {
	for i := 0; i < length; i++ {
		sc.shift ^= uint32(sc.rand.myRand() * 255.0)
		idx := uint32(str[i] ^ byte(sc.shift))
		str[i] = sc.decodeBuff[idx]
		sc.shift ^= uint32(str[i])
	}
}

//SQLDecode Function to handle the decode() function
func SQLDecode(str string, password string) (string, error) {
	var sc sqlCrypt

	strByte := []byte(str)
	passwdByte := []byte(password)

	sc.init(passwdByte, len(passwdByte))
	sc.decode(strByte, len(strByte))

	return string(strByte), nil
}

// SQLEncode Function to handle the encode() function
func SQLEncode(cryptStr string, password string) (string, error) {
	var sc sqlCrypt

	cryptStrByte := []byte(cryptStr)
	passwdByte := []byte(password)

	sc.init(passwdByte, len(passwdByte))
	sc.encode(cryptStrByte, len(cryptStrByte))

	return string(cryptStrByte), nil
}
