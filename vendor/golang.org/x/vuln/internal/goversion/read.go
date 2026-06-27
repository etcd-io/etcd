// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package goversion reports the Go version used to build program executables.
//
// This is a copy of rsc.io/goversion/version. We renamed the package to goversion
// to differentiate between version package in standard library that we also use.
package goversion

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Version is the information reported by ReadExe.
type Version struct {
	Release        string // Go version (runtime.Version in the program)
	ModuleInfo     string // program's module information
	BoringCrypto   bool   // program uses BoringCrypto
	StandardCrypto bool   // program uses standard crypto (replaced by BoringCrypto)
	FIPSOnly       bool   // program imports "crypto/tls/fipsonly"
}

// ReadExe reports information about the Go version used to build
// the program executable named by file.
func ReadExe(file string) (Version, error) {
	var v Version
	f, err := openExe(file)
	if err != nil {
		return v, err
	}
	defer f.Close()
	isGo := false
	for _, name := range f.SectionNames() {
		if name == ".note.go.buildid" {
			isGo = true
		}
	}
	syms, symsErr := f.Symbols()
	isGccgo := false
	for _, sym := range syms {
		name := sym.Name
		if name == "runtime.main" || name == "main.main" {
			isGo = true
		}
		if strings.HasPrefix(name, "runtime.") && strings.HasSuffix(name, "$descriptor") {
			isGccgo = true
		}
		if name == "runtime.buildVersion" {
			isGo = true
			release, err := readBuildVersion(f, sym.Addr, sym.Size)
			if err != nil {
				return v, err
			}
			v.Release = release
		}
		// Note: Using strings.HasPrefix because Go 1.17+ adds ".abi0" to many of these symbols.
		if strings.Contains(name, "_Cfunc__goboringcrypto_") || strings.HasPrefix(name, "crypto/internal/boring/sig.BoringCrypto") {
			v.BoringCrypto = true
		}
		if strings.HasPrefix(name, "crypto/internal/boring/sig.FIPSOnly") {
			v.FIPSOnly = true
		}
		for _, re := range standardCryptoNames {
			if re.MatchString(name) {
				v.StandardCrypto = true
			}
		}
		if strings.HasPrefix(name, "crypto/internal/boring/sig.StandardCrypto") {
			v.StandardCrypto = true
		}
	}

	if DebugMatch {
		v.Release = ""
	}
	if err := findModuleInfo(&v, f); err != nil {
		return v, err
	}
	if v.Release == "" {
		g, release := readBuildVersionX86Asm(f)
		if g {
			isGo = true
			v.Release = release
			if err := findCryptoSigs(&v, f); err != nil {
				return v, err
			}
		}
	}
	if isGccgo && v.Release == "" {
		isGo = true
		v.Release = "gccgo (version unknown)"
	}
	if !isGo && symsErr != nil {
		return v, symsErr
	}

	if !isGo {
		return v, errors.New("not a Go executable")
	}
	if v.Release == "" {
		v.Release = "unknown Go version"
	}
	return v, nil
}

var re = regexp.MustCompile

var standardCryptoNames = []*regexp.Regexp{
	re(`^crypto/sha1\.\(\*digest\)`),
	re(`^crypto/sha256\.\(\*digest\)`),
	re(`^crypto/rand\.\(\*devReader\)`),
	re(`^crypto/rsa\.encrypt(\.abi.)?$`),
	re(`^crypto/rsa\.decrypt(\.abi.)?$`),
}

func readBuildVersion(f exe, addr, size uint64) (string, error) {
	if size == 0 {
		size = uint64(f.AddrSize() * 2)
	}
	if size != 8 && size != 16 {
		return "", fmt.Errorf("invalid size for runtime.buildVersion")
	}
	data, err := f.ReadData(addr, size)
	if err != nil {
		return "", fmt.Errorf("reading runtime.buildVersion: %v", err)
	}

	if size == 8 {
		addr = uint64(f.ByteOrder().Uint32(data))
		size = uint64(f.ByteOrder().Uint32(data[4:]))
	} else {
		addr = f.ByteOrder().Uint64(data)
		size = f.ByteOrder().Uint64(data[8:])
	}
	if size > 1000 {
		return "", fmt.Errorf("implausible string size %d for runtime.buildVersion", size)
	}

	data, err = f.ReadData(addr, size)
	if err != nil {
		return "", fmt.Errorf("reading runtime.buildVersion string data: %v", err)
	}
	return string(data), nil
}

// Code signatures that indicate BoringCrypto or crypto/internal/fipsonly.
// These are not byte literals in order to avoid the actual
// byte signatures appearing in the goversion binary,
// because on some systems you can't tell rodata from text.
var (
	sigBoringCrypto, _   = hex.DecodeString("EB1DF448F44BF4B332F52813A3B450D441CC2485F001454E92101B1D2F1950C3")
	sigStandardCrypto, _ = hex.DecodeString("EB1DF448F44BF4BAEE4DFA9851CA56A91145E83E99C59CF911CB8E80DAF12FC3")
	sigFIPSOnly, _       = hex.DecodeString("EB1DF448F44BF4363CB9CE9D68047D31F28D325D5CA5873F5D80CAF6D6151BC3")
)

func findCryptoSigs(v *Version, f exe) error {
	const maxSigLen = 1 << 10
	start, end := f.TextRange()
	for addr := start; addr < end; {
		size := uint64(1 << 20)
		if end-addr < size {
			size = end - addr
		}
		data, err := f.ReadData(addr, size)
		if err != nil {
			return fmt.Errorf("reading text: %v", err)
		}
		if haveSig(data, sigBoringCrypto) {
			v.BoringCrypto = true
		}
		if haveSig(data, sigFIPSOnly) {
			v.FIPSOnly = true
		}
		if haveSig(data, sigStandardCrypto) {
			v.StandardCrypto = true
		}
		if addr+size < end {
			size -= maxSigLen
		}
		addr += size
	}
	return nil
}

func haveSig(data, sig []byte) bool {
	const align = 16
	for {
		i := bytes.Index(data, sig)
		if i < 0 {
			return false
		}
		if i&(align-1) == 0 {
			return true
		}
		// Found unaligned match; unexpected but
		// skip to next aligned boundary and keep searching.
		data = data[(i+align-1)&^(align-1):]
	}
}

func findModuleInfo(v *Version, f exe) error {
	const maxModInfo = 128 << 10
	start, end := f.RODataRange()
	for addr := start; addr < end; {
		size := uint64(4 << 20)
		if end-addr < size {
			size = end - addr
		}
		data, err := f.ReadData(addr, size)
		if err != nil {
			return fmt.Errorf("reading text: %v", err)
		}
		if haveModuleInfo(data, v) {
			return nil
		}
		if addr+size < end {
			size -= maxModInfo
		}
		addr += size
	}
	return nil
}

var (
	infoStart, _ = hex.DecodeString("3077af0c9274080241e1c107e6d618e6")
	infoEnd, _   = hex.DecodeString("f932433186182072008242104116d8f2")
)

func haveModuleInfo(data []byte, v *Version) bool {
	i := bytes.Index(data, infoStart)
	if i < 0 {
		return false
	}
	j := bytes.Index(data[i:], infoEnd)
	if j < 0 {
		return false
	}
	v.ModuleInfo = string(data[i+len(infoStart) : i+j])
	return true
}
