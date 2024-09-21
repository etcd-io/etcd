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

const magic uint32 = 0xED0CDAED

type inBucket struct {
	root     uint64 // page id of the bucket's root-level page
	sequence uint64 // monotonically incrementing, used by NextSequence()
}

type meta struct {
	magic    uint32
	version  uint32
	pageSize uint32
	flags    uint32
	root     inBucket
	freelist uint64
	pgid     uint64
	txid     uint64
	checksum uint64
}

func loadPageMeta(buf []byte) *meta {
	return (*meta)(unsafe.Pointer(&buf[pageHeaderSize]))
}
