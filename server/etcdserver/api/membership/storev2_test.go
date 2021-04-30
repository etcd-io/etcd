// Copyright 2021 The etcd Authors
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

package membership

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.uber.org/zap/zaptest"
)

func TestIsMetaStoreOnly(t *testing.T) {
	lg := zaptest.NewLogger(t)
	s := v2store.New("/0", "/1")

	metaOnly, err := IsMetaStoreOnly(s)
	assert.NoError(t, err)
	assert.True(t, metaOnly, "Just created v2store should be meta-only")

	mustSaveClusterVersionToStore(lg, s, semver.New("3.5.17"))
	metaOnly, err = IsMetaStoreOnly(s)
	assert.NoError(t, err)
	assert.True(t, metaOnly, "Just created v2store should be meta-only")

	mustSaveMemberToStore(lg, s, &Member{ID: 0x00abcd})
	metaOnly, err = IsMetaStoreOnly(s)
	assert.NoError(t, err)
	assert.True(t, metaOnly, "Just created v2store should be meta-only")

	_, err = s.Create("/1/foo", false, "v1", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent})
	assert.NoError(t, err)
	metaOnly, err = IsMetaStoreOnly(s)
	assert.NoError(t, err)
	assert.False(t, metaOnly, "Just created v2store should be meta-only")

	_, err = s.Delete("/1/foo", false, false)
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.False(t, metaOnly, "Just created v2store should be meta-only")
}
