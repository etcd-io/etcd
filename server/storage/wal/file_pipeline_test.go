// Copyright 2018 The etcd Authors
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

package wal

import (
	"math"
	"testing"

	"go.uber.org/zap/zaptest"
)

func TestFilePipeline(t *testing.T) {
	tdir := t.TempDir()

	fp := newFilePipeline(zaptest.NewLogger(t), tdir, SegmentSizeBytes)
	defer fp.Close()

	f, ferr := fp.Open()
	if ferr != nil {
		t.Fatal(ferr)
	}
	f.Close()
}

func TestFilePipelineFailPreallocate(t *testing.T) {
	tdir := t.TempDir()

	fp := newFilePipeline(zaptest.NewLogger(t), tdir, math.MaxInt64)
	defer fp.Close()

	f, ferr := fp.Open()
	if f != nil || ferr == nil { // no space left on device
		t.Fatal("expected error on invalid pre-allocate size, but no error")
	}
}
