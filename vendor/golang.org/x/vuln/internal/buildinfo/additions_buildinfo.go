// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.18

package buildinfo

// This file adds to buildinfo the functionality for extracting the PCLN table.

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrNoSymbols represents non-existence of symbol
// table in binaries supported by buildinfo.
var ErrNoSymbols = errors.New("no symbol section")

// SymbolInfo is derived from cmd/internal/objfile/elf.go:symbols, symbolData.
func (x *elfExe) SymbolInfo(name string) (uint64, uint64, io.ReaderAt, error) {
	sym, err := x.lookupSymbol(name)
	if err != nil || sym == nil {
		if errors.Is(err, elf.ErrNoSymbols) {
			return 0, 0, nil, ErrNoSymbols
		}
		return 0, 0, nil, fmt.Errorf("no symbol %q", name)
	}
	prog := x.progContaining(sym.Value)
	if prog == nil {
		return 0, 0, nil, fmt.Errorf("no Prog containing value %d for %q", sym.Value, name)
	}
	return sym.Value, prog.Vaddr, prog.ReaderAt, nil
}

func (x *elfExe) lookupSymbol(name string) (*elf.Symbol, error) {
	x.symbolsOnce.Do(func() {
		syms, err := x.f.Symbols()
		if err != nil {
			x.symbolsErr = err
			return
		}
		x.symbols = make(map[string]*elf.Symbol, len(syms))
		for _, s := range syms {
			s := s // make a copy to prevent aliasing
			x.symbols[s.Name] = &s
		}
	})
	if x.symbolsErr != nil {
		return nil, x.symbolsErr
	}
	return x.symbols[name], nil
}

func (x *elfExe) progContaining(addr uint64) *elf.Prog {
	for _, p := range x.f.Progs {
		if addr >= p.Vaddr && addr < p.Vaddr+p.Filesz {
			return p
		}
	}
	return nil
}

const go12magic = 0xfffffffb
const go116magic = 0xfffffffa

// PCLNTab is derived from cmd/internal/objfile/elf.go:pcln.
func (x *elfExe) PCLNTab() ([]byte, uint64) {
	var offset uint64
	text := x.f.Section(".text")
	if text != nil {
		offset = text.Offset
	}
	pclntab := x.f.Section(".gopclntab")
	if pclntab == nil {
		// Addition: this code is added to support some form of stripping.
		pclntab = x.f.Section(".data.rel.ro.gopclntab")
		if pclntab == nil {
			pclntab = x.f.Section(".data.rel.ro")
			if pclntab == nil {
				return nil, 0
			}
			// Possibly the PCLN table has been stuck in the .data.rel.ro section, but without
			// its own section header. We can search for for the start by looking for the four
			// byte magic and the go magic.
			b, err := pclntab.Data()
			if err != nil {
				return nil, 0
			}
			// TODO(rolandshoemaker): I'm not sure if the 16 byte increment during the search is
			// actually correct. During testing it worked, but that may be because I got lucky
			// with the binary I was using, and we need to do four byte jumps to exhaustively
			// search the section?
			for i := 0; i < len(b); i += 16 {
				if len(b)-i > 16 && b[i+4] == 0 && b[i+5] == 0 &&
					(b[i+6] == 1 || b[i+6] == 2 || b[i+6] == 4) &&
					(b[i+7] == 4 || b[i+7] == 8) {
					// Also check for the go magic
					leMagic := binary.LittleEndian.Uint32(b[i:])
					beMagic := binary.BigEndian.Uint32(b[i:])
					switch {
					case leMagic == go12magic:
						fallthrough
					case beMagic == go12magic:
						fallthrough
					case leMagic == go116magic:
						fallthrough
					case beMagic == go116magic:
						return b[i:], offset
					}
				}
			}
		}
	}
	b, err := pclntab.Data()
	if err != nil {
		return nil, 0
	}
	return b, offset
}

// SymbolInfo is derived from cmd/internal/objfile/pe.go:findPESymbol, loadPETable.
func (x *peExe) SymbolInfo(name string) (uint64, uint64, io.ReaderAt, error) {
	sym, err := x.lookupSymbol(name)
	if err != nil {
		return 0, 0, nil, err
	}
	if sym == nil {
		return 0, 0, nil, fmt.Errorf("no symbol %q", name)
	}
	sect := x.f.Sections[sym.SectionNumber-1]
	// In PE, the symbol's value is the offset from the section start.
	return uint64(sym.Value), 0, sect.ReaderAt, nil
}

func (x *peExe) lookupSymbol(name string) (*pe.Symbol, error) {
	x.symbolsOnce.Do(func() {
		x.symbols = make(map[string]*pe.Symbol, len(x.f.Symbols))
		if len(x.f.Symbols) == 0 {
			x.symbolsErr = ErrNoSymbols
			return
		}
		for _, s := range x.f.Symbols {
			x.symbols[s.Name] = s
		}
	})
	if x.symbolsErr != nil {
		return nil, x.symbolsErr
	}
	return x.symbols[name], nil
}

// PCLNTab is derived from cmd/internal/objfile/pe.go:pcln.
// Assumes that the underlying symbol table exists, otherwise
// it might panic.
func (x *peExe) PCLNTab() ([]byte, uint64) {
	var textOffset uint64
	for _, section := range x.f.Sections {
		if section.Name == ".text" {
			textOffset = uint64(section.Offset)
			break
		}
	}

	var start, end int64
	var section int
	if s, _ := x.lookupSymbol("runtime.pclntab"); s != nil {
		start = int64(s.Value)
		section = int(s.SectionNumber - 1)
	}
	if s, _ := x.lookupSymbol("runtime.epclntab"); s != nil {
		end = int64(s.Value)
	}
	if start == 0 || end == 0 {
		return nil, 0
	}
	offset := int64(x.f.Sections[section].Offset) + start
	size := end - start

	pclntab := make([]byte, size)
	if _, err := x.r.ReadAt(pclntab, offset); err != nil {
		return nil, 0
	}
	return pclntab, textOffset
}

// SymbolInfo is derived from cmd/internal/objfile/macho.go:symbols.
func (x *machoExe) SymbolInfo(name string) (uint64, uint64, io.ReaderAt, error) {
	sym, err := x.lookupSymbol(name)
	if err != nil {
		return 0, 0, nil, err
	}
	if sym == nil {
		return 0, 0, nil, fmt.Errorf("no symbol %q", name)
	}
	seg := x.segmentContaining(sym.Value)
	if seg == nil {
		return 0, 0, nil, fmt.Errorf("no Segment containing value %d for %q", sym.Value, name)
	}
	return sym.Value, seg.Addr, seg.ReaderAt, nil
}

func (x *machoExe) lookupSymbol(name string) (*macho.Symbol, error) {
	const mustExistSymbol = "runtime.main"
	x.symbolsOnce.Do(func() {
		x.symbols = make(map[string]*macho.Symbol, len(x.f.Symtab.Syms))
		for _, s := range x.f.Symtab.Syms {
			s := s // make a copy to prevent aliasing
			x.symbols[s.Name] = &s
		}
		// In the presence of stripping, the symbol table for darwin
		// binaries will not be empty, but the program symbols will
		// be missing.
		if _, ok := x.symbols[mustExistSymbol]; !ok {
			x.symbolsErr = ErrNoSymbols
		}
	})

	if x.symbolsErr != nil {
		return nil, x.symbolsErr
	}
	return x.symbols[name], nil
}

func (x *machoExe) segmentContaining(addr uint64) *macho.Segment {
	for _, load := range x.f.Loads {
		seg, ok := load.(*macho.Segment)
		if ok && seg.Addr <= addr && addr <= seg.Addr+seg.Filesz-1 && seg.Name != "__PAGEZERO" {
			return seg
		}
	}
	return nil
}

// SymbolInfo is derived from cmd/internal/objfile/macho.go:pcln.
func (x *machoExe) PCLNTab() ([]byte, uint64) {
	var textOffset uint64
	text := x.f.Section("__text")
	if text != nil {
		textOffset = uint64(text.Offset)
	}
	pclntab := x.f.Section("__gopclntab")
	if pclntab == nil {
		return nil, 0
	}
	b, err := pclntab.Data()
	if err != nil {
		return nil, 0
	}
	return b, textOffset
}
