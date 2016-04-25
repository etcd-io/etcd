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
	"compress/gzip"
	"io"
	"io/ioutil"
)

// NewGzipCompressor creates a Compressor based on gzip.
func NewGzipCompressor() *GzipCompressor {
	return &GzipCompressor{}
}

type GzipCompressor struct{}

func (c *GzipCompressor) Do(w io.Writer, p []byte) error {
	var writer *gzip.Writer
	if wp := gzipWriterPool.Get(); wp != nil {
		writer = wp.(*gzip.Writer)
		writer.Reset(w)
	} else {
		writer = gzip.NewWriter(w)
	}
	if _, err := writer.Write(p); err != nil {
		return err
	}
	err := writer.Close()
	gzipWriterPool.Put(writer)
	return err
}

func (c *GzipCompressor) Type() string {
	return headers[Gzip]
}

// NewGzipDecompressor creates a Compressor based on gzip.
func NewGzipDecompressor() *GzipDecompressor {
	return &GzipDecompressor{}
}

type GzipDecompressor struct{}

func (c *GzipDecompressor) Do(r io.Reader) ([]byte, error) {
	var reader *gzip.Reader
	if wp := gzipReaderPool.Get(); wp != nil {
		reader = wp.(*gzip.Reader)
		reader.Reset(r)
	} else {
		var err error
		reader, err = gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
	}
	defer func() {
		reader.Close()
		gzipReaderPool.Put(reader)
	}()
	return ioutil.ReadAll(reader)
}

func (c *GzipDecompressor) Type() string {
	return headers[Gzip]
}
