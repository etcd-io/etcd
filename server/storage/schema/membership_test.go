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

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/version"
	serverversion "go.etcd.io/etcd/server/v3/etcdserver/version"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

func TestDowngradeInfoFromBackend(t *testing.T) {
	lg := zaptest.NewLogger(t)
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)

	mbe := NewMembershipBackend(lg, be)

	mbe.MustCreateBackendBuckets()
	mbe.be.ForceCommit()
	assert.Nil(t, mbe.DowngradeInfoFromBackend())

	dinfo := &serverversion.DowngradeInfo{Enabled: true, TargetVersion: version.V3_5.String()}
	mbe.MustSaveDowngradeToBackend(dinfo)

	info := mbe.DowngradeInfoFromBackend()

	assert.Equal(t, dinfo, info)
}
