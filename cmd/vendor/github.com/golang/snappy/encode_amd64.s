// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !appengine
// +build gc
// +build !noasm

#include "textflag.h"

// TODO: figure out why the XXX lines compile with Go 1.4 and Go tip but not
// Go 1.6.
//
// This is https://github.com/golang/snappy/issues/29

// The asm code generally follows the pure Go code in encode_other.go, except
// where marked with a "!!!".

// ----------------------------------------------------------------------------

// func emitLiteral(dst, lit []byte) int
//
// All local variables fit into registers. The register allocation:
//	- AX	return value
//	- BX	n
//	- CX	len(lit)
//	- SI	&lit[0]
//	- DI	&dst[i]
//
// The 24 bytes of stack space is to call runtime·memmove.
TEXT ·emitLiteral(SB), NOSPLIT, $24-56
	MOVQ dst_base+0(FP), DI
	MOVQ lit_base+24(FP), SI
	MOVQ lit_len+32(FP), CX
	MOVQ CX, AX
	MOVL CX, BX
	SUBL $1, BX

	CMPL BX, $60
	JLT  oneByte
	CMPL BX, $256
	JLT  twoBytes

threeBytes:
	MOVB $0xf4, 0(DI)
	MOVW BX, 1(DI)
	ADDQ $3, DI
	ADDQ $3, AX
	JMP  emitLiteralEnd

twoBytes:
	MOVB $0xf0, 0(DI)
	MOVB BX, 1(DI)
	ADDQ $2, DI
	ADDQ $2, AX
	JMP  emitLiteralEnd

oneByte:
	SHLB $2, BX
	MOVB BX, 0(DI)
	ADDQ $1, DI
	ADDQ $1, AX

emitLiteralEnd:
	MOVQ AX, ret+48(FP)

	// copy(dst[i:], lit)
	//
	// This means calling runtime·memmove(&dst[i], &lit[0], len(lit)), so we push
	// DI, SI and CX as arguments.
	MOVQ DI, 0(SP)
	MOVQ SI, 8(SP)
	MOVQ CX, 16(SP)
	CALL runtime·memmove(SB)
	RET

// ----------------------------------------------------------------------------

// func emitCopy(dst []byte, offset, length int) int
//
// All local variables fit into registers. The register allocation:
//	- BX	offset
//	- CX	length
//	- SI	&dst[0]
//	- DI	&dst[i]
TEXT ·emitCopy(SB), NOSPLIT, $0-48
	MOVQ dst_base+0(FP), DI
	MOVQ DI, SI
	MOVQ offset+24(FP), BX
	MOVQ length+32(FP), CX

loop0:
	// for length >= 68 { etc }
	CMPL CX, $68
	JLT  step1

	// Emit a length 64 copy, encoded as 3 bytes.
	MOVB $0xfe, 0(DI)
	MOVW BX, 1(DI)
	ADDQ $3, DI
	SUBL $64, CX
	JMP  loop0

step1:
	// if length > 64 { etc }
	CMPL CX, $64
	JLE  step2

	// Emit a length 60 copy, encoded as 3 bytes.
	MOVB $0xee, 0(DI)
	MOVW BX, 1(DI)
	ADDQ $3, DI
	SUBL $60, CX

step2:
	// if length >= 12 || offset >= 2048 { goto step3 }
	CMPL CX, $12
	JGE  step3
	CMPL BX, $2048
	JGE  step3

	// Emit the remaining copy, encoded as 2 bytes.
	MOVB BX, 1(DI)
	SHRL $8, BX
	SHLB $5, BX
	SUBB $4, CX
	SHLB $2, CX
	ORB  CX, BX
	ORB  $1, BX
	MOVB BX, 0(DI)
	ADDQ $2, DI

	// Return the number of bytes written.
	SUBQ SI, DI
	MOVQ DI, ret+40(FP)
	RET

step3:
	// Emit the remaining copy, encoded as 3 bytes.
	SUBL $1, CX
	SHLB $2, CX
	ORB  $2, CX
	MOVB CX, 0(DI)
	MOVW BX, 1(DI)
	ADDQ $3, DI

	// Return the number of bytes written.
	SUBQ SI, DI
	MOVQ DI, ret+40(FP)
	RET

// ----------------------------------------------------------------------------

// func extendMatch(src []byte, i, j int) int
//
// All local variables fit into registers. The register allocation:
//	- CX	&src[0]
//	- DX	&src[len(src)]
//	- SI	&src[i]
//	- DI	&src[j]
//	- R9	&src[len(src) - 8]
TEXT ·extendMatch(SB), NOSPLIT, $0-48
	MOVQ src_base+0(FP), CX
	MOVQ src_len+8(FP), DX
	MOVQ i+24(FP), SI
	MOVQ j+32(FP), DI
	ADDQ CX, DX
	ADDQ CX, SI
	ADDQ CX, DI
	MOVQ DX, R9
	SUBQ $8, R9

cmp8:
	// As long as we are 8 or more bytes before the end of src, we can load and
	// compare 8 bytes at a time. If those 8 bytes are equal, repeat.
	CMPQ DI, R9
	JA   cmp1
	MOVQ (SI), AX
	MOVQ (DI), BX
	CMPQ AX, BX
	JNE  bsf
	ADDQ $8, SI
	ADDQ $8, DI
	JMP  cmp8

bsf:
	// If those 8 bytes were not equal, XOR the two 8 byte values, and return
	// the index of the first byte that differs. The BSF instruction finds the
	// least significant 1 bit, the amd64 architecture is little-endian, and
	// the shift by 3 converts a bit index to a byte index.
	XORQ AX, BX
	BSFQ BX, BX
	SHRQ $3, BX
	ADDQ BX, DI

	// Convert from &src[ret] to ret.
	SUBQ CX, DI
	MOVQ DI, ret+40(FP)
	RET

cmp1:
	// In src's tail, compare 1 byte at a time.
	CMPQ DI, DX
	JAE  extendMatchEnd
	MOVB (SI), AX
	MOVB (DI), BX
	CMPB AX, BX
	JNE  extendMatchEnd
	ADDQ $1, SI
	ADDQ $1, DI
	JMP  cmp1

extendMatchEnd:
	// Convert from &src[ret] to ret.
	SUBQ CX, DI
	MOVQ DI, ret+40(FP)
	RET

// ----------------------------------------------------------------------------

// func encodeBlock(dst, src []byte) (d int)
//
// All local variables fit into registers, other than "var table". The register
// allocation:
//	- AX	.	.
//	- BX	.	.
//	- CX	56	shift (note that amd64 shifts by non-immediates must use CX).
//	- DX	64	&src[0], tableSize
//	- SI	72	&src[s]
//	- DI	80	&dst[d]
//	- R9	88	sLimit
//	- R10	.	&src[nextEmit]
//	- R11	96	prevHash, currHash, nextHash, offset
//	- R12	104	&src[base], skip
//	- R13	.	&src[nextS]
//	- R14	.	len(src), bytesBetweenHashLookups, x
//	- R15	112	candidate
//
// The second column (56, 64, etc) is the stack offset to spill the registers
// when calling other functions. We could pack this slightly tighter, but it's
// simpler to have a dedicated spill map independent of the function called.
//
// "var table [maxTableSize]uint16" takes up 32768 bytes of stack space. An
// extra 56 bytes, to call other functions, and an extra 64 bytes, to spill
// local variables (registers) during calls gives 32768 + 56 + 64 = 32888.
TEXT ·encodeBlock(SB), 0, $32888-56
	MOVQ dst_base+0(FP), DI
	MOVQ src_base+24(FP), SI
	MOVQ src_len+32(FP), R14

	// shift, tableSize := uint32(32-8), 1<<8
	MOVQ $24, CX
	MOVQ $256, DX

calcShift:
	// for ; tableSize < maxTableSize && tableSize < len(src); tableSize *= 2 {
	//	shift--
	// }
	CMPQ DX, $16384
	JGE  varTable
	CMPQ DX, R14
	JGE  varTable
	SUBQ $1, CX
	SHLQ $1, DX
	JMP  calcShift

varTable:
	// var table [maxTableSize]uint16
	//
	// In the asm code, unlike the Go code, we can zero-initialize only the
	// first tableSize elements. Each uint16 element is 2 bytes and each MOVOU
	// writes 16 bytes, so we can do only tableSize/8 writes instead of the
	// 2048 writes that would zero-initialize all of table's 32768 bytes.
	SHRQ $3, DX
	LEAQ table-32768(SP), BX
	PXOR X0, X0

memclr:
	MOVOU X0, 0(BX)
	ADDQ  $16, BX
	SUBQ  $1, DX
	JNZ   memclr

	// !!! DX = &src[0]
	MOVQ SI, DX

	// sLimit := len(src) - inputMargin
	MOVQ R14, R9
	SUBQ $15, R9

	// !!! Pre-emptively spill CX, DX and R9 to the stack. Their values don't
	// change for the rest of the function.
	MOVQ CX, 56(SP)
	MOVQ DX, 64(SP)
	MOVQ R9, 88(SP)

	// nextEmit := 0
	MOVQ DX, R10

	// s := 1
	ADDQ $1, SI

	// nextHash := hash(load32(src, s), shift)
	MOVL  0(SI), R11
	IMULL $0x1e35a7bd, R11
	SHRL  CX, R11

outer:
	// for { etc }

	// skip := 32
	MOVQ $32, R12

	// nextS := s
	MOVQ SI, R13

	// candidate := 0
	MOVQ $0, R15

inner0:
	// for { etc }

	// s := nextS
	MOVQ R13, SI

	// bytesBetweenHashLookups := skip >> 5
	MOVQ R12, R14
	SHRQ $5, R14

	// nextS = s + bytesBetweenHashLookups
	ADDQ R14, R13

	// skip += bytesBetweenHashLookups
	ADDQ R14, R12

	// if nextS > sLimit { goto emitRemainder }
	MOVQ R13, AX
	SUBQ DX, AX
	CMPQ AX, R9
	JA   emitRemainder

	// candidate = int(table[nextHash])
	// XXX: MOVWQZX table-32768(SP)(R11*2), R15
	// XXX: 4e 0f b7 7c 5c 78       movzwq 0x78(%rsp,%r11,2),%r15
	BYTE $0x4e
	BYTE $0x0f
	BYTE $0xb7
	BYTE $0x7c
	BYTE $0x5c
	BYTE $0x78

	// table[nextHash] = uint16(s)
	MOVQ SI, AX
	SUBQ DX, AX
	// XXX: MOVW AX, table-32768(SP)(R11*2)
	// XXX: 66 42 89 44 5c 78       mov    %ax,0x78(%rsp,%r11,2)
	BYTE $0x66
	BYTE $0x42
	BYTE $0x89
	BYTE $0x44
	BYTE $0x5c
	BYTE $0x78

	// nextHash = hash(load32(src, nextS), shift)
	MOVL  0(R13), R11
	IMULL $0x1e35a7bd, R11
	SHRL  CX, R11

	// if load32(src, s) != load32(src, candidate) { continue } break
	MOVL 0(SI), AX
	MOVL (DX)(R15*1), BX
	CMPL AX, BX
	JNE  inner0

fourByteMatch:
	// As per the encode_other.go code:
	//
	// A 4-byte match has been found. We'll later see etc.

	// !!! Jump to a fast path for short (<= 16 byte) literals. See the comment
	// on inputMargin in encode.go.
	MOVQ SI, AX
	SUBQ R10, AX
	CMPQ AX, $16
	JLE  emitLiteralFastPath

	// d += emitLiteral(dst[d:], src[nextEmit:s])
	//
	// Push args.
	MOVQ DI, 0(SP)
	MOVQ $0, 8(SP)   // Unnecessary, as the callee ignores it, but conservative.
	MOVQ $0, 16(SP)  // Unnecessary, as the callee ignores it, but conservative.
	MOVQ R10, 24(SP)
	MOVQ AX, 32(SP)
	MOVQ AX, 40(SP)  // Unnecessary, as the callee ignores it, but conservative.

	// Spill local variables (registers) onto the stack; call; unspill.
	MOVQ SI, 72(SP)
	MOVQ DI, 80(SP)
	MOVQ R15, 112(SP)
	CALL ·emitLiteral(SB)
	MOVQ 56(SP), CX
	MOVQ 64(SP), DX
	MOVQ 72(SP), SI
	MOVQ 80(SP), DI
	MOVQ 88(SP), R9
	MOVQ 112(SP), R15

	// Finish the "d +=" part of "d += emitLiteral(etc)".
	ADDQ 48(SP), DI
	JMP  inner1

emitLiteralFastPath:
	// !!! Emit the 1-byte encoding "uint8(len(lit)-1)<<2".
	MOVB AX, BX
	SUBB $1, BX
	SHLB $2, BX
	MOVB BX, (DI)
	ADDQ $1, DI

	// !!! Implement the copy from lit to dst as a 16-byte load and store.
	// (Encode's documentation says that dst and src must not overlap.)
	//
	// This always copies 16 bytes, instead of only len(lit) bytes, but that's
	// OK. Subsequent iterations will fix up the overrun.
	//
	// Note that on amd64, it is legal and cheap to issue unaligned 8-byte or
	// 16-byte loads and stores. This technique probably wouldn't be as
	// effective on architectures that are fussier about alignment.
	MOVOU 0(R10), X0
	MOVOU X0, 0(DI)
	ADDQ  AX, DI

inner1:
	// for { etc }

	// base := s
	MOVQ SI, R12

	// !!! offset := base - candidate
	MOVQ R12, R11
	SUBQ R15, R11
	SUBQ DX, R11

	// s = extendMatch(src, candidate+4, s+4)
	//
	// Push args.
	MOVQ DX, 0(SP)
	MOVQ src_len+32(FP), R14
	MOVQ R14, 8(SP)
	MOVQ R14, 16(SP)         // Unnecessary, as the callee ignores it, but conservative.
	ADDQ $4, R15
	MOVQ R15, 24(SP)
	ADDQ $4, SI
	SUBQ DX, SI
	MOVQ SI, 32(SP)

	// Spill local variables (registers) onto the stack; call; unspill.
	//
	// We don't need to unspill CX or R9 as we are just about to call another
	// function.
	MOVQ DI, 80(SP)
	MOVQ R11, 96(SP)
	MOVQ R12, 104(SP)
	CALL ·extendMatch(SB)
	MOVQ 64(SP), DX
	MOVQ 80(SP), DI
	MOVQ 96(SP), R11
	MOVQ 104(SP), R12

	// Finish the "s =" part of "s = extendMatch(etc)", remembering that the SI
	// register holds &src[s], not s.
	MOVQ 40(SP), SI
	ADDQ DX, SI

	// d += emitCopy(dst[d:], base-candidate, s-base)
	//
	// Push args.
	MOVQ DI, 0(SP)
	MOVQ $0, 8(SP)   // Unnecessary, as the callee ignores it, but conservative.
	MOVQ $0, 16(SP)  // Unnecessary, as the callee ignores it, but conservative.
	MOVQ R11, 24(SP)
	MOVQ SI, AX
	SUBQ R12, AX
	MOVQ AX, 32(SP)

	// Spill local variables (registers) onto the stack; call; unspill.
	MOVQ SI, 72(SP)
	MOVQ DI, 80(SP)
	CALL ·emitCopy(SB)
	MOVQ 56(SP), CX
	MOVQ 64(SP), DX
	MOVQ 72(SP), SI
	MOVQ 80(SP), DI
	MOVQ 88(SP), R9

	// Finish the "d +=" part of "d += emitCopy(etc)".
	ADDQ 40(SP), DI

	// nextEmit = s
	MOVQ SI, R10

	// if s >= sLimit { goto emitRemainder }
	MOVQ SI, AX
	SUBQ DX, AX
	CMPQ AX, R9
	JAE  emitRemainder

	// As per the encode_other.go code:
	//
	// We could immediately etc.

	// x := load64(src, s-1)
	MOVQ -1(SI), R14

	// prevHash := hash(uint32(x>>0), shift)
	MOVL  R14, R11
	IMULL $0x1e35a7bd, R11
	SHRL  CX, R11

	// table[prevHash] = uint16(s-1)
	MOVQ SI, AX
	SUBQ DX, AX
	SUBQ $1, AX
	// XXX: MOVW AX, table-32768(SP)(R11*2)
	// XXX: 66 42 89 44 5c 78       mov    %ax,0x78(%rsp,%r11,2)
	BYTE $0x66
	BYTE $0x42
	BYTE $0x89
	BYTE $0x44
	BYTE $0x5c
	BYTE $0x78

	// currHash := hash(uint32(x>>8), shift)
	SHRQ  $8, R14
	MOVL  R14, R11
	IMULL $0x1e35a7bd, R11
	SHRL  CX, R11

	// candidate = int(table[currHash])
	// XXX: MOVWQZX table-32768(SP)(R11*2), R15
	// XXX: 4e 0f b7 7c 5c 78       movzwq 0x78(%rsp,%r11,2),%r15
	BYTE $0x4e
	BYTE $0x0f
	BYTE $0xb7
	BYTE $0x7c
	BYTE $0x5c
	BYTE $0x78

	// table[currHash] = uint16(s)
	ADDQ $1, AX
	// XXX: MOVW AX, table-32768(SP)(R11*2)
	// XXX: 66 42 89 44 5c 78       mov    %ax,0x78(%rsp,%r11,2)
	BYTE $0x66
	BYTE $0x42
	BYTE $0x89
	BYTE $0x44
	BYTE $0x5c
	BYTE $0x78

	// if uint32(x>>8) == load32(src, candidate) { continue }
	MOVL (DX)(R15*1), BX
	CMPL R14, BX
	JEQ  inner1

	// nextHash = hash(uint32(x>>16), shift)
	SHRQ  $8, R14
	MOVL  R14, R11
	IMULL $0x1e35a7bd, R11
	SHRL  CX, R11

	// s++
	ADDQ $1, SI

	// break out of the inner1 for loop, i.e. continue the outer loop.
	JMP outer

emitRemainder:
	// if nextEmit < len(src) { etc }
	MOVQ src_len+32(FP), AX
	ADDQ DX, AX
	CMPQ R10, AX
	JEQ  encodeBlockEnd

	// d += emitLiteral(dst[d:], src[nextEmit:])
	//
	// Push args.
	MOVQ DI, 0(SP)
	MOVQ $0, 8(SP)   // Unnecessary, as the callee ignores it, but conservative.
	MOVQ $0, 16(SP)  // Unnecessary, as the callee ignores it, but conservative.
	MOVQ R10, 24(SP)
	SUBQ R10, AX
	MOVQ AX, 32(SP)
	MOVQ AX, 40(SP)  // Unnecessary, as the callee ignores it, but conservative.

	// Spill local variables (registers) onto the stack; call; unspill.
	MOVQ DI, 80(SP)
	CALL ·emitLiteral(SB)
	MOVQ 80(SP), DI

	// Finish the "d +=" part of "d += emitLiteral(etc)".
	ADDQ 48(SP), DI

encodeBlockEnd:
	MOVQ dst_base+0(FP), AX
	SUBQ AX, DI
	MOVQ DI, d+48(FP)
	RET
