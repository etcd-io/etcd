// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goversion

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type sym struct {
	Name string
	Addr uint64
	Size uint64
}

type exe interface {
	AddrSize() int // bytes
	ReadData(addr, size uint64) ([]byte, error)
	Symbols() ([]sym, error)
	SectionNames() []string
	Close() error
	ByteOrder() binary.ByteOrder
	Entry() uint64
	TextRange() (uint64, uint64)
	RODataRange() (uint64, uint64)
}

func openExe(file string) (exe, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	data := make([]byte, 16)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, err
	}
	f.Seek(0, 0)
	if bytes.HasPrefix(data, []byte("\x7FELF")) {
		e, err := elf.NewFile(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &elfExe{f, e}, nil
	}
	if bytes.HasPrefix(data, []byte("MZ")) {
		e, err := pe.NewFile(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &peExe{f, e}, nil
	}
	if bytes.HasPrefix(data, []byte("\xFE\xED\xFA")) || bytes.HasPrefix(data[1:], []byte("\xFA\xED\xFE")) {
		e, err := macho.NewFile(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &machoExe{f, e}, nil
	}
	return nil, fmt.Errorf("unrecognized executable format")
}

type elfExe struct {
	os *os.File
	f  *elf.File
}

func (x *elfExe) AddrSize() int { return 0 }

func (x *elfExe) ByteOrder() binary.ByteOrder { return x.f.ByteOrder }

func (x *elfExe) Close() error {
	return x.os.Close()
}

func (x *elfExe) Entry() uint64 { return x.f.Entry }

func (x *elfExe) ReadData(addr, size uint64) ([]byte, error) {
	for _, prog := range x.f.Progs {
		// The following line was commented from the original code.
		//fmt.Printf("%#x %#x %#x\n", addr, prog.Vaddr, prog.Vaddr+prog.Filesz)
		if prog.Vaddr <= addr && addr <= prog.Vaddr+prog.Filesz-1 {
			n := prog.Vaddr + prog.Filesz - addr
			if n > size {
				n = size
			}
			data := make([]byte, n)
			_, err := prog.ReadAt(data, int64(addr-prog.Vaddr))
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("address not mapped")
}

func (x *elfExe) Symbols() ([]sym, error) {
	syms, err := x.f.Symbols()
	if err != nil {
		return nil, err
	}
	var out []sym
	for _, s := range syms {
		out = append(out, sym{s.Name, s.Value, s.Size})
	}
	return out, nil
}

func (x *elfExe) SectionNames() []string {
	var names []string
	for _, sect := range x.f.Sections {
		names = append(names, sect.Name)
	}
	return names
}

func (x *elfExe) TextRange() (uint64, uint64) {
	for _, p := range x.f.Progs {
		if p.Type == elf.PT_LOAD && p.Flags&elf.PF_X != 0 {
			return p.Vaddr, p.Vaddr + p.Filesz
		}
	}
	return 0, 0
}

func (x *elfExe) RODataRange() (uint64, uint64) {
	for _, p := range x.f.Progs {
		if p.Type == elf.PT_LOAD && p.Flags&(elf.PF_R|elf.PF_W|elf.PF_X) == elf.PF_R {
			return p.Vaddr, p.Vaddr + p.Filesz
		}
	}
	for _, p := range x.f.Progs {
		if p.Type == elf.PT_LOAD && p.Flags&(elf.PF_R|elf.PF_W|elf.PF_X) == (elf.PF_R|elf.PF_X) {
			return p.Vaddr, p.Vaddr + p.Filesz
		}
	}
	return 0, 0
}

type peExe struct {
	os *os.File
	f  *pe.File
}

func (x *peExe) imageBase() uint64 {
	switch oh := x.f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		return uint64(oh.ImageBase)
	case *pe.OptionalHeader64:
		return oh.ImageBase
	}
	return 0
}

func (x *peExe) AddrSize() int {
	if x.f.Machine == pe.IMAGE_FILE_MACHINE_AMD64 {
		return 8
	}
	return 4
}

func (x *peExe) ByteOrder() binary.ByteOrder { return binary.LittleEndian }

func (x *peExe) Close() error {
	return x.os.Close()
}

func (x *peExe) Entry() uint64 {
	switch oh := x.f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		return uint64(oh.ImageBase + oh.AddressOfEntryPoint)
	case *pe.OptionalHeader64:
		return oh.ImageBase + uint64(oh.AddressOfEntryPoint)
	}
	return 0
}

func (x *peExe) ReadData(addr, size uint64) ([]byte, error) {
	addr -= x.imageBase()
	data := make([]byte, size)
	for _, sect := range x.f.Sections {
		if uint64(sect.VirtualAddress) <= addr && addr+size-1 <= uint64(sect.VirtualAddress+sect.Size-1) {
			_, err := sect.ReadAt(data, int64(addr-uint64(sect.VirtualAddress)))
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("address not mapped")
}

func (x *peExe) Symbols() ([]sym, error) {
	base := x.imageBase()
	var out []sym
	for _, s := range x.f.Symbols {
		if s.SectionNumber <= 0 || int(s.SectionNumber) > len(x.f.Sections) {
			continue
		}
		sect := x.f.Sections[s.SectionNumber-1]
		out = append(out, sym{s.Name, uint64(s.Value) + base + uint64(sect.VirtualAddress), 0})
	}
	return out, nil
}

func (x *peExe) SectionNames() []string {
	var names []string
	for _, sect := range x.f.Sections {
		names = append(names, sect.Name)
	}
	return names
}

func (x *peExe) TextRange() (uint64, uint64) {
	// Assume text is first non-empty section.
	for _, sect := range x.f.Sections {
		if sect.VirtualAddress != 0 && sect.Size != 0 {
			return uint64(sect.VirtualAddress) + x.imageBase(), uint64(sect.VirtualAddress+sect.Size) + x.imageBase()
		}
	}
	return 0, 0
}

func (x *peExe) RODataRange() (uint64, uint64) {
	return x.TextRange()
}

type machoExe struct {
	os *os.File
	f  *macho.File
}

func (x *machoExe) AddrSize() int {
	if x.f.Cpu&0x01000000 != 0 {
		return 8
	}
	return 4
}

func (x *machoExe) ByteOrder() binary.ByteOrder { return x.f.ByteOrder }

func (x *machoExe) Close() error {
	return x.os.Close()
}

func (x *machoExe) Entry() uint64 {
	for _, load := range x.f.Loads {
		b, ok := load.(macho.LoadBytes)
		if !ok {
			continue
		}
		// TODO: Other thread states.
		bo := x.f.ByteOrder
		const x86_THREAD_STATE64 = 4
		cmd, siz := macho.LoadCmd(bo.Uint32(b[0:4])), bo.Uint32(b[4:8])
		if cmd == macho.LoadCmdUnixThread && siz == 184 && bo.Uint32(b[8:12]) == x86_THREAD_STATE64 {
			return bo.Uint64(b[144:])
		}
	}
	return 0
}

func (x *machoExe) ReadData(addr, size uint64) ([]byte, error) {
	data := make([]byte, size)
	for _, load := range x.f.Loads {
		seg, ok := load.(*macho.Segment)
		if !ok {
			continue
		}
		if seg.Addr <= addr && addr+size-1 <= seg.Addr+seg.Filesz-1 {
			if seg.Name == "__PAGEZERO" {
				continue
			}
			_, err := seg.ReadAt(data, int64(addr-seg.Addr))
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("address not mapped")
}

func (x *machoExe) Symbols() ([]sym, error) {
	var out []sym
	for _, s := range x.f.Symtab.Syms {
		out = append(out, sym{s.Name, s.Value, 0})
	}
	return out, nil
}

func (x *machoExe) SectionNames() []string {
	var names []string
	for _, sect := range x.f.Sections {
		names = append(names, sect.Name)
	}
	return names
}

func (x *machoExe) TextRange() (uint64, uint64) {
	// Assume text is first non-empty segment.
	for _, load := range x.f.Loads {
		seg, ok := load.(*macho.Segment)
		if ok && seg.Name != "__PAGEZERO" && seg.Addr != 0 && seg.Filesz != 0 {
			return seg.Addr, seg.Addr + seg.Filesz
		}
	}
	return 0, 0
}

func (x *machoExe) RODataRange() (uint64, uint64) {
	return x.TextRange()
}
