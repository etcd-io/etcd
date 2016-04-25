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

	"github.com/golang/snappy"
)

// NewSnappyCompressor creates a Compressor based on github.com/golang/snappy.
func NewSnappyCompressor() *SnappyCompressor {
	return &SnappyCompressor{}
}

type SnappyCompressor struct{}

func (c *SnappyCompressor) Do(w io.Writer, p []byte) error {
	var writer *snappy.Writer
	if wp := snappyWriterPool.Get(); wp != nil {
		writer = wp.(*snappy.Writer)
		writer.Reset(w)
	} else {
		writer = snappy.NewBufferedWriter(w)
	}
	if _, err := writer.Write(p); err != nil {
		return err
	}
	err := writer.Close()
	snappyWriterPool.Put(writer)
	return err
}

func (c *SnappyCompressor) Type() string {
	return headers[Snappy]
}

// NewSnappyDecompressor creates a Decompressor based on github.com/golang/snappy.
func NewSnappyDecompressor() *SnappyDecompressor {
	return &SnappyDecompressor{}
}

type SnappyDecompressor struct{}

func (c *SnappyDecompressor) Do(r io.Reader) ([]byte, error) {
	var reader *snappy.Reader
	if wp := snappyReaderPool.Get(); wp != nil {
		reader = wp.(*snappy.Reader)
		reader.Reset(r)
	} else {
		reader = snappy.NewReader(r)
	}
	defer snappyReaderPool.Put(reader)
	return ioutil.ReadAll(reader)
}

func (c *SnappyDecompressor) Type() string {
	return headers[Snappy]
}
