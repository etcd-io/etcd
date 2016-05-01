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

type Type int

const (
	NoCompress Type = iota
	Snappy
)

var (
	headers = [...]string{
		"",
		"snappy",
	}
)

func (ct Type) String() string {
	return headers[ct]
}

func ParseType(opt string) Type {
	switch opt {
	case "snappy":
		return Snappy

	default:
		return NoCompress
	}
}

type Compressor interface {
	// Do compresses p into w.
	Do(w io.Writer, p []byte) error

	// Type returns the compression algorithm the Compressor uses.
	Type() string
}

func NewCompressor(ct Type) Compressor {
	switch ct {
	case Snappy:
		return NewSnappyCompressor()

	default:
		return nil
	}
}

type Decompressor interface {
	// Do reads the data from r and uncompress them.
	Do(r io.Reader) ([]byte, error)

	// Type returns the compression algorithm the Decompressor uses.
	Type() string
}

func NewDecompressor(ct Type) Decompressor {
	switch ct {
	case Snappy:
		return NewSnappyDecompressor()

	default:
		return nil
	}
}
