// Copyright 2023 The etcd Authors
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

package robustness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRevisionHistory(t *testing.T) {
	rh := revisionHistory{}
	rh.Update(10)
	min, max := rh.Get()
	assert.Equal(t, int64(10), min)
	assert.Equal(t, int64(10), max)

	rh.Update(20)
	min, max = rh.Get()
	assert.Equal(t, int64(10), min)
	assert.Equal(t, int64(20), max)

	rh.Update(-15) // compaction
	min, max = rh.Get()
	assert.Equal(t, int64(15), min)
	assert.Equal(t, int64(20), max)

	rh.Update(-12) // old compaction
	min, max = rh.Get()
	assert.Equal(t, int64(15), min)
	assert.Equal(t, int64(20), max)

	rh.Update(5) // old write
	min, max = rh.Get()
	assert.Equal(t, int64(15), min)
	assert.Equal(t, int64(20), max)
}
