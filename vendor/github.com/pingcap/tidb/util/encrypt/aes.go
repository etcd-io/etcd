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

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"

	"github.com/pingcap/errors"
)

type ecb struct {
	b         cipher.Block
	blockSize int
}

func newECB(b cipher.Block) *ecb {
	return &ecb{
		b:         b,
		blockSize: b.BlockSize(),
	}
}

type ecbEncrypter ecb

// BlockSize implements BlockMode.BlockSize interface.
func (x *ecbEncrypter) BlockSize() int { return x.blockSize }

// CryptBlocks implements BlockMode.CryptBlocks interface.
func (x *ecbEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("ECBEncrypter: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("ECBEncrypter: output smaller than input")
	}
	// See https://en.wikipedia.org/wiki/Block_cipher_mode_of_operation#Electronic_Codebook_.28ECB.29
	for len(src) > 0 {
		x.b.Encrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
	return
}

// newECBEncrypter creates an AES encrypter with ecb mode.
func newECBEncrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbEncrypter)(newECB(b))
}

type ecbDecrypter ecb

// BlockSize implements BlockMode.BlockSize interface.
func (x *ecbDecrypter) BlockSize() int { return x.blockSize }

// CryptBlocks implements BlockMode.CryptBlocks interface.
func (x *ecbDecrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("ECBDecrypter: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("ECBDecrypter: output smaller than input")
	}
	// See https://en.wikipedia.org/wiki/Block_cipher_mode_of_operation#Electronic_Codebook_.28ECB.29
	for len(src) > 0 {
		x.b.Decrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}

func newECBDecrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbDecrypter)(newECB(b))
}

// PKCS7Pad pads data using PKCS7.
// See hhttp://tools.ietf.org/html/rfc2315.
func PKCS7Pad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	padLen := blockSize - (length % blockSize)
	padText := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(data, padText...), nil
}

// PKCS7Unpad unpads data using PKCS7.
// See http://tools.ietf.org/html/rfc2315.
func PKCS7Unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("Invalid padding size")
	}
	if length%blockSize != 0 {
		return nil, errors.New("Invalid padding size")
	}
	pad := data[length-1]
	padLen := int(pad)
	if padLen > blockSize || padLen == 0 {
		return nil, errors.New("Invalid padding size")
	}
	// TODO: Fix timing attack here.
	for _, v := range data[length-padLen : length-1] {
		if v != pad {
			return nil, errors.New("Invalid padding")
		}
	}
	return data[:length-padLen], nil
}

// AESEncryptWithECB encrypts data using AES with ECB mode.
func AESEncryptWithECB(str, key []byte) ([]byte, error) {
	cb, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Trace(err)
	}
	mode := newECBEncrypter(cb)
	return aesEncrypt(str, mode)
}

// AESDecryptWithECB decrypts data using AES with ECB mode.
func AESDecryptWithECB(cryptStr, key []byte) ([]byte, error) {
	cb, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Trace(err)
	}
	mode := newECBDecrypter(cb)
	return aesDecrypt(cryptStr, mode)
}

// DeriveKeyMySQL derives the encryption key from a password in MySQL algorithm.
// See https://security.stackexchange.com/questions/4863/mysql-aes-encrypt-key-length.
func DeriveKeyMySQL(key []byte, blockSize int) []byte {
	rKey := make([]byte, blockSize)
	rIdx := 0
	for _, k := range key {
		if rIdx == blockSize {
			rIdx = 0
		}
		rKey[rIdx] ^= k
		rIdx++
	}
	return rKey
}

// AESEncryptWithCBC encrypts data using AES with CBC mode.
func AESEncryptWithCBC(str, key []byte, iv []byte) ([]byte, error) {
	cb, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Trace(err)
	}
	mode := cipher.NewCBCEncrypter(cb, iv)
	return aesEncrypt(str, mode)
}

// AESDecryptWithCBC decrypts data using AES with CBC mode.
func AESDecryptWithCBC(cryptStr, key []byte, iv []byte) ([]byte, error) {
	cb, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Trace(err)
	}
	mode := cipher.NewCBCDecrypter(cb, iv)
	return aesDecrypt(cryptStr, mode)
}

// aesDecrypt decrypts data using AES.
func aesDecrypt(cryptStr []byte, mode cipher.BlockMode) ([]byte, error) {
	blockSize := mode.BlockSize()
	if len(cryptStr)%blockSize != 0 {
		return nil, errors.New("Corrupted data")
	}
	data := make([]byte, len(cryptStr))
	mode.CryptBlocks(data, cryptStr)
	plain, err := PKCS7Unpad(data, blockSize)
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// aesEncrypt encrypts data using AES.
func aesEncrypt(str []byte, mode cipher.BlockMode) ([]byte, error) {
	blockSize := mode.BlockSize()
	// The str arguments can be any length, and padding is automatically added to
	// str so it is a multiple of a block as required by block-based algorithms such as AES.
	// This padding is automatically removed by the AES_DECRYPT() function.
	data, err := PKCS7Pad(str, blockSize)
	if err != nil {
		return nil, err
	}
	crypted := make([]byte, len(data))
	mode.CryptBlocks(crypted, data)
	return crypted, nil
}
