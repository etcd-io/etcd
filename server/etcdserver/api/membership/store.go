// Copyright 2016 The etcd Authors
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
	"encoding/json"
	"fmt"
	"path"

	"go.etcd.io/etcd/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/mvcc/backend"

	"github.com/coreos/go-semver/semver"
	"go.uber.org/zap"
)

const (
	attributesSuffix     = "attributes"
	raftAttributesSuffix = "raftAttributes"

	// the prefix for stroing membership related information in store provided by store pkg.
	storePrefix = "/0"
)

var (
	membersBucketName        = []byte("members")
	membersRemovedBucketName = []byte("members_removed")
	clusterBucketName        = []byte("cluster")

	StoreMembersPrefix        = path.Join(storePrefix, "members")
	storeRemovedMembersPrefix = path.Join(storePrefix, "removed_members")
)

func mustSaveMemberToBackend(lg *zap.Logger, be backend.Backend, m *Member) {
	mkey := backendMemberKey(m.ID)
	mvalue, err := json.Marshal(m)
	if err != nil {
		lg.Panic("failed to marshal member", zap.Error(err))
	}

	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	tx.UnsafePut(membersBucketName, mkey, mvalue)
}

func mustDeleteMemberFromBackend(be backend.Backend, id types.ID) {
	mkey := backendMemberKey(id)

	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	tx.UnsafeDelete(membersBucketName, mkey)
	tx.UnsafePut(membersRemovedBucketName, mkey, []byte("removed"))
}

func mustSaveClusterVersionToBackend(be backend.Backend, ver *semver.Version) {
	ckey := backendClusterVersionKey()

	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	tx.UnsafePut(clusterBucketName, ckey, []byte(ver.String()))
}

func mustSaveDowngradeToBackend(lg *zap.Logger, be backend.Backend, downgrade *DowngradeInfo) {
	dkey := backendDowngradeKey()
	dvalue, err := json.Marshal(downgrade)
	if err != nil {
		lg.Panic("failed to marshal downgrade information", zap.Error(err))
	}
	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	tx.UnsafePut(clusterBucketName, dkey, dvalue)
}

func mustSaveMemberToStore(lg *zap.Logger, s v2store.Store, m *Member) {
	b, err := json.Marshal(m.RaftAttributes)
	if err != nil {
		lg.Panic("failed to marshal raftAttributes", zap.Error(err))
	}
	p := path.Join(MemberStoreKey(m.ID), raftAttributesSuffix)
	if _, err := s.Create(p, false, string(b), false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		lg.Panic(
			"failed to save member to store",
			zap.String("path", p),
			zap.Error(err),
		)
	}
}

func mustDeleteMemberFromStore(lg *zap.Logger, s v2store.Store, id types.ID) {
	if _, err := s.Delete(MemberStoreKey(id), true, true); err != nil {
		lg.Panic(
			"failed to delete member from store",
			zap.String("path", MemberStoreKey(id)),
			zap.Error(err),
		)
	}
	if _, err := s.Create(RemovedMemberStoreKey(id), false, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		lg.Panic(
			"failed to create removedMember",
			zap.String("path", RemovedMemberStoreKey(id)),
			zap.Error(err),
		)
	}
}

func mustUpdateMemberInStore(lg *zap.Logger, s v2store.Store, m *Member) {
	b, err := json.Marshal(m.RaftAttributes)
	if err != nil {
		lg.Panic("failed to marshal raftAttributes", zap.Error(err))
	}
	p := path.Join(MemberStoreKey(m.ID), raftAttributesSuffix)
	if _, err := s.Update(p, string(b), v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		lg.Panic(
			"failed to update raftAttributes",
			zap.String("path", p),
			zap.Error(err),
		)
	}
}

func mustUpdateMemberAttrInStore(lg *zap.Logger, s v2store.Store, m *Member) {
	b, err := json.Marshal(m.Attributes)
	if err != nil {
		lg.Panic("failed to marshal attributes", zap.Error(err))
	}
	p := path.Join(MemberStoreKey(m.ID), attributesSuffix)
	if _, err := s.Set(p, false, string(b), v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		lg.Panic(
			"failed to update attributes",
			zap.String("path", p),
			zap.Error(err),
		)
	}
}

func mustSaveClusterVersionToStore(lg *zap.Logger, s v2store.Store, ver *semver.Version) {
	if _, err := s.Set(StoreClusterVersionKey(), false, ver.String(), v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		lg.Panic(
			"failed to save cluster version to store",
			zap.String("path", StoreClusterVersionKey()),
			zap.Error(err),
		)
	}
}

// nodeToMember builds member from a key value node.
// the child nodes of the given node MUST be sorted by key.
func nodeToMember(lg *zap.Logger, n *v2store.NodeExtern) (*Member, error) {
	m := &Member{ID: MustParseMemberIDFromKey(lg, n.Key)}
	attrs := make(map[string][]byte)
	raftAttrKey := path.Join(n.Key, raftAttributesSuffix)
	attrKey := path.Join(n.Key, attributesSuffix)
	for _, nn := range n.Nodes {
		if nn.Key != raftAttrKey && nn.Key != attrKey {
			return nil, fmt.Errorf("unknown key %q", nn.Key)
		}
		attrs[nn.Key] = []byte(*nn.Value)
	}
	if data := attrs[raftAttrKey]; data != nil {
		if err := json.Unmarshal(data, &m.RaftAttributes); err != nil {
			return nil, fmt.Errorf("unmarshal raftAttributes error: %v", err)
		}
	} else {
		return nil, fmt.Errorf("raftAttributes key doesn't exist")
	}
	if data := attrs[attrKey]; data != nil {
		if err := json.Unmarshal(data, &m.Attributes); err != nil {
			return m, fmt.Errorf("unmarshal attributes error: %v", err)
		}
	}
	return m, nil
}

func backendMemberKey(id types.ID) []byte {
	return []byte(id.String())
}

func backendClusterVersionKey() []byte {
	return []byte("clusterVersion")
}

func backendDowngradeKey() []byte {
	return []byte("downgrade")
}

func mustCreateBackendBuckets(be backend.Backend) {
	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	tx.UnsafeCreateBucket(membersBucketName)
	tx.UnsafeCreateBucket(membersRemovedBucketName)
	tx.UnsafeCreateBucket(clusterBucketName)
}

func MemberStoreKey(id types.ID) string {
	return path.Join(StoreMembersPrefix, id.String())
}

func StoreClusterVersionKey() string {
	return path.Join(storePrefix, "version")
}

func MemberAttributesStorePath(id types.ID) string {
	return path.Join(MemberStoreKey(id), attributesSuffix)
}

func MustParseMemberIDFromKey(lg *zap.Logger, key string) types.ID {
	id, err := types.IDFromString(path.Base(key))
	if err != nil {
		lg.Panic("failed to parse memver id from key", zap.Error(err))
	}
	return id
}

func RemovedMemberStoreKey(id types.ID) string {
	return path.Join(storeRemovedMembersPrefix, id.String())
}
