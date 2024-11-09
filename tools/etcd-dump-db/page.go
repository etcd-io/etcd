// Copyright 2024 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import "unsafe"

const (
	pageHeaderSize      = unsafe.Sizeof(page{})
	leafPageElementSize = unsafe.Sizeof(leafPageElement{})
	pageMaxAllocSize    = 0xFFFFFFF
)

const (
	leafPageFlag = 0x02
)

type page struct {
	id       uint64
	flags    uint16
	count    uint16
	overflow uint32
}

func (p *page) isLeafPage() bool {
	return p.flags == leafPageFlag
}

func loadPage(buf []byte) *page {
	return (*page)(unsafe.Pointer(&buf[0]))
}

// leafPageElement retrieves the leaf node by index
func (p *page) leafPageElement(index uint16) *leafPageElement {
	return (*leafPageElement)(unsafeIndex(unsafe.Pointer(p), unsafe.Sizeof(*p),
		leafPageElementSize, int(index)))
}

// leafPageElement represents a node on a leaf page.
type leafPageElement struct {
	flags uint32
	pos   uint32
	ksize uint32
	vsize uint32
}

// Key returns a byte slice of the node key.
func (n *leafPageElement) key() []byte {
	i := int(n.pos)
	j := i + int(n.ksize)
	return unsafeByteSlice(unsafe.Pointer(n), 0, i, j)
}

// Value returns a byte slice of the node value.
func (n *leafPageElement) value() []byte {
	i := int(n.pos) + int(n.ksize)
	j := i + int(n.vsize)
	return unsafeByteSlice(unsafe.Pointer(n), 0, i, j)
}
