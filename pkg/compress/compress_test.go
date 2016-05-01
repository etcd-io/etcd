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
	"bytes"
	"strings"
	"testing"
)

func TestCompressPlainBytes(t *testing.T) {
	var (
		bytesN = 2 * 1024 * 1024 // 2MB
		tBts   = []byte(strings.Repeat("a", bytesN))
	)
	for _, tp := range []Type{Snappy} {
		wr := new(bytes.Buffer)
		cp := NewCompressor(tp)
		if err := cp.Do(wr, tBts); err != nil {
			t.Fatal(err)
		}

		compressed := wr.Bytes()
		dp := NewDecompressor(tp)
		decompressed, err := dp.Do(bytes.NewReader(compressed))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(tBts, decompressed) {
			t.Fatalf("expected equal, got %q != %q", tBts, decompressed)
		}
	}
}
