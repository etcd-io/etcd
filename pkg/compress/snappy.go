// Copyright 2016 CoreOS, Inc.
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

package compress

import (
	"io"
	"io/ioutil"
	"sync"

	"github.com/golang/snappy"
)

var (
	snappyWriterPool sync.Pool
	snappyReaderPool sync.Pool
)

func NewSnappyCompressor() Compressor {
	return snappyCompressor{}
}

type snappyCompressor struct{}

func (_ snappyCompressor) Do(w io.Writer, p []byte) error {
	var writer *snappy.Writer
	if wp := snappyWriterPool.Get(); wp != nil {
		writer = wp.(*snappy.Writer)
		writer.Reset(w)
	} else {
		writer = snappy.NewBufferedWriter(w)
	}
	if _, err := writer.Write(p); err != nil {
		writer.Close()
		snappyWriterPool.Put(writer)
		return err
	}
	werr := writer.Close()
	snappyWriterPool.Put(writer)
	return werr
}

func (_ snappyCompressor) Type() string {
	return headers[Snappy]
}

func NewSnappyDecompressor() Decompressor {
	return snappyDecompressor{}
}

type snappyDecompressor struct{}

func (_ snappyDecompressor) Do(r io.Reader) ([]byte, error) {
	var reader *snappy.Reader
	if wp := snappyReaderPool.Get(); wp != nil {
		reader = wp.(*snappy.Reader)
		reader.Reset(r)
	} else {
		reader = snappy.NewReader(r)
	}
	bts, err := ioutil.ReadAll(reader)
	snappyReaderPool.Put(reader)
	return bts, err
}

func (_ snappyDecompressor) Type() string {
	return headers[Snappy]
}
