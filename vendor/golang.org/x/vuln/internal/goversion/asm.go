// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goversion

import (
	"encoding/binary"
	"fmt"
	"os"
)

type matcher [][]uint32

const (
	pWild    uint32 = 0xff00
	pAddr    uint32 = 0x10000
	pEnd     uint32 = 0x20000
	pRelAddr uint32 = 0x30000

	opMaybe = 1 + iota
	opMust
	opDone
	opAnchor = 0x100
	opSub8   = 0x200
	opFlags  = opAnchor | opSub8
)

var amd64Matcher = matcher{
	{opMaybe | opAnchor,
		// __rt0_amd64_darwin:
		//	JMP __rt0_amd64
		0xe9, pWild | pAddr, pWild, pWild, pWild | pEnd, 0xcc, 0xcc, 0xcc,
	},
	{opMaybe,
		// _rt0_amd64_linux:
		//	lea 0x8(%rsp), %rsi
		//	mov (%rsp), %rdi
		//	lea ADDR(%rip), %rax # main
		//	jmpq *%rax
		0x48, 0x8d, 0x74, 0x24, 0x08,
		0x48, 0x8b, 0x3c, 0x24, 0x48,
		0x8d, 0x05, pWild | pAddr, pWild, pWild, pWild | pEnd,
		0xff, 0xe0,
	},
	{opMaybe,
		// _rt0_amd64_linux:
		//	lea 0x8(%rsp), %rsi
		//	mov (%rsp), %rdi
		//	mov $ADDR, %eax # main
		//	jmpq *%rax
		0x48, 0x8d, 0x74, 0x24, 0x08,
		0x48, 0x8b, 0x3c, 0x24,
		0xb8, pWild | pAddr, pWild, pWild, pWild,
		0xff, 0xe0,
	},
	{opMaybe,
		// __rt0_amd64:
		//	mov (%rsp), %rdi
		//	lea 8(%rsp), %rsi
		//	jmp runtime.rt0_g0
		0x48, 0x8b, 0x3c, 0x24,
		0x48, 0x8d, 0x74, 0x24, 0x08,
		0xe9, pWild | pAddr, pWild, pWild, pWild | pEnd,
		0xcc, 0xcc,
	},
	{opMaybe,
		// _start (toward end)
		//	lea __libc_csu_fini(%rip), %r8
		//	lea __libc_csu_init(%rip), %rcx
		//	lea ADDR(%rip), %rdi # main
		//	callq *xxx(%rip)
		0x4c, 0x8d, 0x05, pWild, pWild, pWild, pWild,
		0x48, 0x8d, 0x0d, pWild, pWild, pWild, pWild,
		0x48, 0x8d, 0x3d, pWild | pAddr, pWild, pWild, pWild | pEnd,
		0xff, 0x15,
	},
	{opMaybe,
		// _start (toward end)
		//	push %rsp (1)
		//	mov $__libc_csu_fini, %r8 (7)
		//	mov $__libc_csu_init, %rcx (7)
		//	mov $ADDR, %rdi # main (7)
		//	callq *xxx(%rip)
		0x54,
		0x49, 0xc7, 0xc0, pWild, pWild, pWild, pWild,
		0x48, 0xc7, 0xc1, pWild, pWild, pWild, pWild,
		0x48, 0xc7, 0xc7, pAddr | pWild, pWild, pWild, pWild,
	},
	{opMaybe | opAnchor,
		// main:
		//	lea ADDR(%rip), %rax # rt0_go
		//	jmpq *%rax
		0x48, 0x8d, 0x05, pWild | pAddr, pWild, pWild, pWild | pEnd,
		0xff, 0xe0,
	},
	{opMaybe | opAnchor,
		// main:
		//	mov $ADDR, %eax
		//	jmpq *%rax
		0xb8, pWild | pAddr, pWild, pWild, pWild,
		0xff, 0xe0,
	},
	{opMaybe | opAnchor,
		// main:
		//	JMP runtime.rt0_go(SB)
		0xe9, pWild | pAddr, pWild, pWild, pWild | pEnd, 0xcc, 0xcc, 0xcc,
	},
	{opMust | opAnchor,
		// rt0_go:
		//	mov %rdi, %rax
		//	mov %rsi, %rbx
		//	sub %0x27, %rsp
		//	and $0xfffffffffffffff0,%rsp
		//	mov %rax,0x10(%rsp)
		//	mov %rbx,0x18(%rsp)
		0x48, 0x89, 0xf8,
		0x48, 0x89, 0xf3,
		0x48, 0x83, 0xec, 0x27,
		0x48, 0x83, 0xe4, 0xf0,
		0x48, 0x89, 0x44, 0x24, 0x10,
		0x48, 0x89, 0x5c, 0x24, 0x18,
	},
	{opMust,
		// later in rt0_go:
		//	mov %eax, (%rsp)
		//	mov 0x18(%rsp), %rax
		//	mov %rax, 0x8(%rsp)
		//	callq runtime.args
		//	callq runtime.osinit
		//	callq runtime.schedinit (ADDR)
		0x89, 0x04, 0x24,
		0x48, 0x8b, 0x44, 0x24, 0x18,
		0x48, 0x89, 0x44, 0x24, 0x08,
		0xe8, pWild, pWild, pWild, pWild,
		0xe8, pWild, pWild, pWild, pWild,
		0xe8, pWild, pWild, pWild, pWild,
	},
	{opMaybe,
		// later in rt0_go:
		//	mov %eax, (%rsp)
		//	mov 0x18(%rsp), %rax
		//	mov %rax, 0x8(%rsp)
		//	callq runtime.args
		//	callq runtime.osinit
		//	callq runtime.schedinit (ADDR)
		//	lea other(%rip), %rdi
		0x89, 0x04, 0x24,
		0x48, 0x8b, 0x44, 0x24, 0x18,
		0x48, 0x89, 0x44, 0x24, 0x08,
		0xe8, pWild, pWild, pWild, pWild,
		0xe8, pWild, pWild, pWild, pWild,
		0xe8, pWild | pAddr, pWild, pWild, pWild | pEnd,
		0x48, 0x8d, 0x05,
	},
	{opMaybe,
		// later in rt0_go:
		//	mov %eax, (%rsp)
		//	mov 0x18(%rsp), %rax
		//	mov %rax, 0x8(%rsp)
		//	callq runtime.args
		//	callq runtime.osinit
		//	callq runtime.hashinit
		//	callq runtime.schedinit (ADDR)
		//	pushq $main.main
		0x89, 0x04, 0x24,
		0x48, 0x8b, 0x44, 0x24, 0x18,
		0x48, 0x89, 0x44, 0x24, 0x08,
		0xe8, pWild, pWild, pWild, pWild,
		0xe8, pWild, pWild, pWild, pWild,
		0xe8, pWild, pWild, pWild, pWild,
		0xe8, pWild | pAddr, pWild, pWild, pWild | pEnd,
		0x68,
	},
	{opDone | opSub8,
		// schedinit (toward end)
		//	mov ADDR(%rip), %rax
		//	test %rax, %rax
		//	jne <short>
		//	movq $0x7, ADDR(%rip)
		//
		0x48, 0x8b, 0x05, pWild, pWild, pWild, pWild,
		0x48, 0x85, 0xc0,
		0x75, pWild,
		0x48, 0xc7, 0x05, pWild | pAddr, pWild, pWild, pWild, 0x07, 0x00, 0x00, 0x00 | pEnd,
	},
	{opDone | opSub8,
		// schedinit (toward end)
		//	mov ADDR(%rip), %rbx
		//	cmp $0x0, %rbx
		//	jne <short>
		//	lea "unknown"(%rip), %rbx
		//	mov %rbx, ADDR(%rip)
		//	movq $7, (ADDR+8)(%rip)
		0x48, 0x8b, 0x1d, pWild, pWild, pWild, pWild,
		0x48, 0x83, 0xfb, 0x00,
		0x75, pWild,
		0x48, 0x8d, 0x1d, pWild, pWild, pWild, pWild,
		0x48, 0x89, 0x1d, pWild, pWild, pWild, pWild,
		0x48, 0xc7, 0x05, pWild | pAddr, pWild, pWild, pWild, 0x07, 0x00, 0x00, 0x00 | pEnd,
	},
	{opDone,
		// schedinit (toward end)
		//	cmpq $0x0, ADDR(%rip)
		//	jne <short>
		//	lea "unknown"(%rip), %rax
		//	mov %rax, ADDR(%rip)
		//	lea ADDR(%rip), %rax
		//	movq $7, 8(%rax)
		0x48, 0x83, 0x3d, pWild | pAddr, pWild, pWild, pWild, 0x00,
		0x75, pWild,
		0x48, 0x8d, 0x05, pWild, pWild, pWild, pWild,
		0x48, 0x89, 0x05, pWild, pWild, pWild, pWild,
		0x48, 0x8d, 0x05, pWild | pAddr, pWild, pWild, pWild | pEnd,
		0x48, 0xc7, 0x40, 0x08, 0x07, 0x00, 0x00, 0x00,
	},
	{opDone,
		// schedinit (toward end)
		//	cmpq $0x0, ADDR(%rip)
		//	jne <short>
		//	movq $0x7, ADDR(%rip)
		0x48, 0x83, 0x3d, pWild | pAddr, pWild, pWild, pWild, 0x00,
		0x75, pWild,
		0x48, 0xc7, 0x05 | pEnd, pWild | pAddr, pWild, pWild, pWild, 0x07, 0x00, 0x00, 0x00,
	},
	{opDone,
		//	test %eax, %eax
		//	jne <later>
		//	lea "unknown"(RIP), %rax
		//	mov %rax, ADDR(%rip)
		0x48, 0x85, 0xc0, 0x75, pWild, 0x48, 0x8d, 0x05, pWild, pWild, pWild, pWild, 0x48, 0x89, 0x05, pWild | pAddr, pWild, pWild, pWild | pEnd,
	},
	{opDone,
		// schedinit (toward end)
		//	mov ADDR(%rip), %rcx
		//	test %rcx, %rcx
		//	jne <short>
		//	movq $0x7, ADDR(%rip)
		//
		0x48, 0x8b, 0x0d, pWild, pWild, pWild, pWild,
		0x48, 0x85, 0xc9,
		0x75, pWild,
		0x48, 0xc7, 0x05 | pEnd, pWild | pAddr, pWild, pWild, pWild, 0x07, 0x00, 0x00, 0x00,
	},
}

var DebugMatch bool

func (m matcher) match(f exe, addr uint64) (uint64, bool) {
	data, err := f.ReadData(addr, 512)
	if DebugMatch {
		fmt.Fprintf(os.Stderr, "data @%#x: %x\n", addr, data[:16])
	}
	if err != nil {
		if DebugMatch {
			fmt.Fprintf(os.Stderr, "match: %v\n", err)
		}
		return 0, false
	}
	if DebugMatch {
		fmt.Fprintf(os.Stderr, "data: %x\n", data[:32])
	}
Matchers:
	for pc, p := range m {
		op := p[0]
		p = p[1:]
	Search:
		for i := 0; i <= len(data)-len(p); i++ {
			a := -1
			e := -1
			if i > 0 && op&opAnchor != 0 {
				break
			}
			for j := 0; j < len(p); j++ {
				b := byte(p[j])
				m := byte(p[j] >> 8)
				if data[i+j]&^m != b {
					continue Search
				}
				if p[j]&pAddr != 0 {
					a = j
				}
				if p[j]&pEnd != 0 {
					e = j + 1
				}
			}
			// matched
			if DebugMatch {
				fmt.Fprintf(os.Stderr, "match (%d) %#x+%d %x %x\n", pc, addr, i, p, data[i:i+len(p)])
			}
			if a != -1 {
				val := uint64(int32(binary.LittleEndian.Uint32(data[i+a:])))
				if e == -1 {
					addr = val
				} else {
					addr += uint64(i+e) + val
				}
				if op&opSub8 != 0 {
					addr -= 8
				}
			}
			if op&^opFlags == opDone {
				if DebugMatch {
					fmt.Fprintf(os.Stderr, "done %x\n", addr)
				}
				return addr, true
			}
			if a != -1 {
				// changed addr, so reload
				data, err = f.ReadData(addr, 512)
				if err != nil {
					return 0, false
				}
				if DebugMatch {
					fmt.Fprintf(os.Stderr, "reload @%#x: %x\n", addr, data[:32])
				}
			}
			continue Matchers
		}
		// not matched
		if DebugMatch {
			fmt.Fprintf(os.Stderr, "no match (%d) %#x %x %x\n", pc, addr, p, data[:32])
		}
		if op&^opFlags == opMust {
			return 0, false
		}
	}
	// ran off end of matcher
	return 0, false
}

func readBuildVersionX86Asm(f exe) (isGo bool, buildVersion string) {
	entry := f.Entry()
	if entry == 0 {
		if DebugMatch {
			fmt.Fprintf(os.Stderr, "missing entry!\n")
		}
		return
	}
	addr, ok := amd64Matcher.match(f, entry)
	if !ok {
		return
	}
	v, err := readBuildVersion(f, addr, 16)
	if err != nil {
		return
	}
	return true, v
}
