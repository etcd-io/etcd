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

import "io"

type Compressor interface {
	Do(w io.Writer, p []byte) error
	Type() string
}

func NewCompressor(ct Type) Compressor {
	switch ct {
	case Gzip:
		return NewGzipCompressor()
	case Snappy:
		return NewSnappyCompressor()
	}
	return nil
}

type Decompressor interface {
	Do(r io.Reader) ([]byte, error)
	Type() string
}

func NewDecompressor(ct Type) Decompressor {
	switch ct {
	case Gzip:
		return NewGzipDecompressor()
	case Snappy:
		return NewSnappyDecompressor()
	}
	return nil
}
