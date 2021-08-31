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

package schema

import (
	"bytes"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/storage/backend"
)

var (
	keyBucketName   = []byte("key")
	metaBucketName  = []byte("meta")
	leaseBucketName = []byte("lease")
	alarmBucketName = []byte("alarm")

	clusterBucketName = []byte("cluster")

	membersBucketName        = []byte("members")
	membersRemovedBucketName = []byte("members_removed")

	authBucketName      = []byte("auth")
	authUsersBucketName = []byte("authUsers")
	authRolesBucketName = []byte("authRoles")

	testBucketName = []byte("test")
)

var (
	Key     = newBucket(1, keyBucketName, true)
	Meta    = newBucket(2, metaBucketName, false)
	Lease   = newBucket(3, leaseBucketName, false)
	Alarm   = newBucket(4, alarmBucketName, false)
	Cluster = newBucket(5, clusterBucketName, false)

	Members        = newBucket(10, membersBucketName, false)
	MembersRemoved = newBucket(11, membersRemovedBucketName, false)

	Auth      = newBucket(20, authBucketName, false)
	AuthUsers = newBucket(21, authUsersBucketName, false)
	AuthRoles = newBucket(22, authRolesBucketName, false)

	Test = newBucket(100, testBucketName, false)

	allBuckets = []backend.Bucket{}
)

func newBucket(id backend.BucketID, name []byte, safeRangeBucket bool) backend.Bucket {
	b := bucket{
		id:              id,
		name:            name,
		safeRangeBucket: safeRangeBucket,
	}
	allBuckets = append(allBuckets, b)
	return b
}

type bucket struct {
	id              backend.BucketID
	name            []byte
	safeRangeBucket bool
}

func (b bucket) ID() backend.BucketID    { return b.id }
func (b bucket) Name() []byte            { return b.name }
func (b bucket) String() string          { return string(b.Name()) }
func (b bucket) IsSafeRangeBucket() bool { return b.safeRangeBucket }

var (
	// Pre v3.5
	ScheduledCompactKeyName    = []byte("scheduledCompactRev")
	FinishedCompactKeyName     = []byte("finishedCompactRev")
	MetaConsistentIndexKeyName = []byte("consistent_index")
	AuthEnabledKeyName         = []byte("authEnabled")
	AuthRevisionKeyName        = []byte("authRevision")
	// Since v3.5
	MetaTermKeyName              = []byte("term")
	MetaConfStateName            = []byte("confState")
	ClusterClusterVersionKeyName = []byte("clusterVersion")
	ClusterDowngradeKeyName      = []byte("downgrade")
	// Since v3.6
	MetaStorageVersionName = []byte("storageVersion")
	// Before adding new meta key please update server/etcdserver/version
)

// DefaultIgnores defines buckets & keys to ignore in hash checking.
func DefaultIgnores(bucket, key []byte) bool {
	// consistent index & term might be changed due to v2 internal sync, which
	// is not controllable by the user.
	// storage version might change after wal snapshot and is not controller by user.
	return bytes.Compare(bucket, Meta.Name()) == 0 &&
		(bytes.Compare(key, MetaTermKeyName) == 0 || bytes.Compare(key, MetaConsistentIndexKeyName) == 0 || bytes.Compare(key, MetaStorageVersionName) == 0)
}

func BackendMemberKey(id types.ID) []byte {
	return []byte(id.String())
}
