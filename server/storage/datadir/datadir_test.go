// Copyright 2022 The etcd Authors
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

package datadir_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/server/v3/storage/datadir"
)

func TestToBackendFileName(t *testing.T) {
	result := datadir.ToBackendFileName("/dir/data-dir")
	assert.Equal(t, "/dir/data-dir/member/snap/db", result)
}

func TestToMemberDir(t *testing.T) {
	result := datadir.ToMemberDir("/dir/data-dir")
	assert.Equal(t, "/dir/data-dir/member", result)
}

func TestToSnapDir(t *testing.T) {
	result := datadir.ToSnapDir("/dir/data-dir")
	assert.Equal(t, "/dir/data-dir/member/snap", result)
}

func TestToWALDir(t *testing.T) {
	result := datadir.ToWALDir("/dir/data-dir")
	assert.Equal(t, "/dir/data-dir/member/wal", result)
}

func TestToWALDirSlash(t *testing.T) {
	result := datadir.ToWALDir("/dir/data-dir/")
	assert.Equal(t, "/dir/data-dir/member/wal", result)
}
