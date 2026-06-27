// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gosym

import (
	"encoding/binary"
	"io"
	"strings"

	sv "golang.org/x/mod/semver"
	"golang.org/x/vuln/internal/semver"
)

const (
	funcSymNameGo119Lower string = "go.func.*"
	funcSymNameGo120      string = "go:func.*"
)

// FuncSymName returns symbol name for Go functions used in binaries
// based on Go version. Supported Go versions are 1.18 and greater.
// If the go version is unreadable it assumes that it is a newer version
// and returns the symbol name for go version 1.20 or greater.
func FuncSymName(goVersion string) string {
	// Support devel goX.Y...
	v := strings.TrimPrefix(goVersion, "devel ")
	v = semver.GoTagToSemver(v)
	mm := sv.MajorMinor(v)
	if sv.Compare(mm, "v1.20") >= 0 || mm == "" {
		return funcSymNameGo120
	} else if sv.Compare(mm, "v1.18") >= 0 {
		return funcSymNameGo119Lower
	}
	return ""
}

// Additions to the original package from cmd/internal/objabi/funcdata.go
const (
	pcdata_InlTreeIndex = 2
	funcdata_InlTree    = 3
)

// InlineTree returns the inline tree for Func f as a sequence of InlinedCalls.
// goFuncValue is the value of the gosym.FuncSymName symbol.
// baseAddr is the address of the memory region (ELF Prog) containing goFuncValue.
// progReader is a ReaderAt positioned at the start of that region.
func (t *LineTable) InlineTree(f *Func, goFuncValue, baseAddr uint64, progReader io.ReaderAt) ([]InlinedCall, error) {
	if f.inlineTreeCount == 0 {
		return nil, nil
	}
	if f.inlineTreeOffset == ^uint32(0) {
		return nil, nil
	}
	var offset int64
	if t.version >= ver118 {
		offset = int64(goFuncValue - baseAddr + uint64(f.inlineTreeOffset))
	} else {
		offset = int64(uint64(f.inlineTreeOffset) - baseAddr)
	}

	r := io.NewSectionReader(progReader, offset, 1<<32) // pick a size larger than we need
	ics := make([]InlinedCall, 0, f.inlineTreeCount)
	for i := 0; i < f.inlineTreeCount; i++ {
		if t.version >= ver120 {
			var ric rawInlinedCall120
			if err := binary.Read(r, t.binary, &ric); err != nil {
				return nil, err
			}
			ics = append(ics, InlinedCall{
				FuncID:   ric.FuncID,
				Name:     t.funcName(uint32(ric.NameOff)),
				ParentPC: ric.ParentPC,
			})
		} else {
			var ric rawInlinedCall112
			if err := binary.Read(r, t.binary, &ric); err != nil {
				return nil, err
			}
			ics = append(ics, InlinedCall{
				FuncID:   ric.FuncID,
				Name:     t.funcName(uint32(ric.Func_)),
				ParentPC: ric.ParentPC,
			})
		}
	}
	return ics, nil
}

// InlinedCall describes a call to an inlined function.
type InlinedCall struct {
	FuncID   uint8  // type of the called function
	Name     string // name of called function
	ParentPC int32  // position of an instruction whose source position is the call site (offset from entry)
}

// rawInlinedCall112 is the encoding of entries in the FUNCDATA_InlTree table
// from Go 1.12 through 1.19. It is equivalent to runtime.inlinedCall.
type rawInlinedCall112 struct {
	Parent   int16 // index of parent in the inltree, or < 0
	FuncID   uint8 // type of the called function
	_        byte
	File     int32 // perCU file index for inlined call. See cmd/link:pcln.go
	Line     int32 // line number of the call site
	Func_    int32 // offset into pclntab for name of called function
	ParentPC int32 // position of an instruction whose source position is the call site (offset from entry)
}

// rawInlinedCall120 is the encoding of entries in the FUNCDATA_InlTree table
// from Go 1.20. It is equivalent to runtime.inlinedCall.
type rawInlinedCall120 struct {
	FuncID    uint8 // type of the called function
	_         [3]byte
	NameOff   int32 // offset into pclntab for name of called function
	ParentPC  int32 // position of an instruction whose source position is the call site (offset from entry)
	StartLine int32 // line number of start of function (func keyword/TEXT directive)
}

func (f funcData) npcdata() uint32 { return f.field(7) }
func (f funcData) nfuncdata(numFuncFields uint32) uint32 {
	return uint32(f.data[f.fieldOffset(numFuncFields-1)+3])
}

func (f funcData) funcdataOffset(i uint8, numFuncFields uint32) uint32 {
	if uint32(i) >= f.nfuncdata(numFuncFields) {
		return ^uint32(0)
	}
	var off uint32
	if f.t.version >= ver118 {
		off = f.fieldOffset(numFuncFields) + // skip fixed part of _func
			f.npcdata()*4 + // skip pcdata
			uint32(i)*4 // index of i'th FUNCDATA
	} else {
		off = f.fieldOffset(numFuncFields) + // skip fixed part of _func
			f.npcdata()*4
		off += uint32(i) * f.t.ptrsize
	}
	return f.t.binary.Uint32(f.data[off:])
}

func (f funcData) fieldOffset(n uint32) uint32 {
	// In Go 1.18, the first field of _func changed
	// from a uintptr entry PC to a uint32 entry offset.
	sz0 := f.t.ptrsize
	if f.t.version >= ver118 {
		sz0 = 4
	}
	return sz0 + (n-1)*4 // subsequent fields are 4 bytes each
}

func (f funcData) pcdataOffset(i uint8, numFuncFields uint32) uint32 {
	if uint32(i) >= f.npcdata() {
		return ^uint32(0)
	}
	off := f.fieldOffset(numFuncFields) + // skip fixed part of _func
		uint32(i)*4 // index of i'th PCDATA
	return f.t.binary.Uint32(f.data[off:])
}

// maxInlineTreeIndexValue returns the maximum value of the inline tree index
// pc-value table in info. This is the only way to determine how many
// IndexedCalls are in an inline tree, since the data of the tree itself is not
// delimited in any way.
func (t *LineTable) maxInlineTreeIndexValue(info funcData, numFuncFields uint32) int {
	if info.npcdata() <= pcdata_InlTreeIndex {
		return -1
	}
	off := info.pcdataOffset(pcdata_InlTreeIndex, numFuncFields)
	p := t.pctab[off:]
	val := int32(-1)
	max := int32(-1)
	var pc uint64
	for t.step(&p, &pc, &val, pc == 0) {
		if val > max {
			max = val
		}
	}
	return int(max)
}

type inlTree struct {
	inlineTreeOffset uint32 // offset from go.func.* symbol
	inlineTreeCount  int    // number of entries in inline tree
}
