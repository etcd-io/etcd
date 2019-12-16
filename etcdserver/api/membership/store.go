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

	"go.etcd.io/etcd/etcdserver/api/v2store"
	"go.etcd.io/etcd/mvcc/backend"
	"go.etcd.io/etcd/pkg/types"
	"go.uber.org/zap"

	"github.com/coreos/go-semver/semver"
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

func mustSaveMemberToBackend(be backend.Backend, m *Member) {
	mkey := backendMemberKey(m.ID)
	mvalue, err := json.Marshal(m)
	if err != nil {
		plog.Panicf("marshal raftAttributes should never fail: %v", err)
	}

	tx := be.BatchTx()
	tx.Lock()
	tx.UnsafePut(membersBucketName, mkey, mvalue)
	tx.Unlock()
}

func mustDeleteMemberFromBackend(be backend.Backend, id types.ID) {
	mkey := backendMemberKey(id)

	tx := be.BatchTx()
	tx.Lock()
	tx.UnsafeDelete(membersBucketName, mkey)
	tx.UnsafePut(membersRemovedBucketName, mkey, []byte("removed"))
	tx.Unlock()
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
		if lg != nil {
			lg.Panic("failed to marshal downgrade information", zap.Error(err))
		}
	}
	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	tx.UnsafePut(clusterBucketName, dkey, dvalue)
}

func mustSaveMemberToStore(s v2store.Store, m *Member) {
	b, err := json.Marshal(m.RaftAttributes)
	if err != nil {
		plog.Panicf("marshal raftAttributes should never fail: %v", err)
	}
	p := path.Join(MemberStoreKey(m.ID), raftAttributesSuffix)
	if _, err := s.Create(p, false, string(b), false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		plog.Panicf("create raftAttributes should never fail: %v", err)
	}
}

func mustDeleteMemberFromStore(s v2store.Store, id types.ID) {
	if _, err := s.Delete(MemberStoreKey(id), true, true); err != nil {
		plog.Panicf("delete member should never fail: %v", err)
	}
	if _, err := s.Create(RemovedMemberStoreKey(id), false, "", false, v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		plog.Panicf("create removedMember should never fail: %v", err)
	}
}

func mustUpdateMemberInStore(s v2store.Store, m *Member) {
	b, err := json.Marshal(m.RaftAttributes)
	if err != nil {
		plog.Panicf("marshal raftAttributes should never fail: %v", err)
	}
	p := path.Join(MemberStoreKey(m.ID), raftAttributesSuffix)
	if _, err := s.Update(p, string(b), v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		plog.Panicf("update raftAttributes should never fail: %v", err)
	}
}

func mustUpdateMemberAttrInStore(s v2store.Store, m *Member) {
	b, err := json.Marshal(m.Attributes)
	if err != nil {
		plog.Panicf("marshal raftAttributes should never fail: %v", err)
	}
	p := path.Join(MemberStoreKey(m.ID), attributesSuffix)
	if _, err := s.Set(p, false, string(b), v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		plog.Panicf("update raftAttributes should never fail: %v", err)
	}
}

func mustSaveClusterVersionToStore(s v2store.Store, ver *semver.Version) {
	if _, err := s.Set(StoreClusterVersionKey(), false, ver.String(), v2store.TTLOptionSet{ExpireTime: v2store.Permanent}); err != nil {
		plog.Panicf("save cluster version should never fail: %v", err)
	}
}

func downgradeInfoFromBackend(lg *zap.Logger, be backend.Backend) *DowngradeInfo {
	dkey := backendDowngradeKey()
	if be != nil {
		tx := be.ReadTx()
		tx.Lock()
		defer tx.Unlock()
		keys, vals := tx.UnsafeRange(clusterBucketName, dkey, nil, 0)
		if len(keys) == 0 {
			return nil
		}

		if len(keys) != 1 {
			if lg != nil {
				lg.Panic(
					"unexpected number of keys when getting cluster version from backend",
					zap.Int("number-of-key", len(keys)),
				)
			}
		}
		var d DowngradeInfo
		if err := json.Unmarshal(vals[0], &d); err != nil {
			if lg != nil {
				lg.Panic("failed to unmarshal downgrade information", zap.Error(err))
			}
		}
		return &d
	}
	return nil
}

func clusterVersionFromBackend(lg *zap.Logger, be backend.Backend) *semver.Version {
	ckey := backendClusterVersionKey()
	tx := be.ReadTx()
	tx.RLock()
	defer tx.RUnlock()
	keys, vals := tx.UnsafeRange(clusterBucketName, ckey, nil, 0)
	if len(keys) == 0 {
		return nil
	}
	if len(keys) != 1 {
		if lg != nil {
			lg.Panic(
				"unexpected number of keys when getting cluster version from backend",
				zap.Int("number-of-key", len(keys)),
			)
		}
	}
	return semver.Must(semver.NewVersion(string(vals[0])))
}

func membersFromStore(lg *zap.Logger, st v2store.Store) (map[types.ID]*Member, map[types.ID]bool) {
	members := make(map[types.ID]*Member)
	removed := make(map[types.ID]bool)
	e, err := st.Get(StoreMembersPrefix, true, true)
	if err != nil {
		if isKeyNotFound(err) {
			return members, removed
		}
		if lg != nil {
			lg.Panic("failed to get members from store", zap.String("path", StoreMembersPrefix), zap.Error(err))
		} else {
			plog.Panicf("get storeMembers should never fail: %v", err)
		}
	}
	for _, n := range e.Node.Nodes {
		var m *Member
		m, err = nodeToMember(n)
		if err != nil {
			if lg != nil {
				lg.Panic("failed to nodeToMember", zap.Error(err))
			} else {
				plog.Panicf("nodeToMember should never fail: %v", err)
			}
		}
		members[m.ID] = m
	}

	e, err = st.Get(storeRemovedMembersPrefix, true, true)
	if err != nil {
		if isKeyNotFound(err) {
			return members, removed
		}
		if lg != nil {
			lg.Panic(
				"failed to get removed members from store",
				zap.String("path", storeRemovedMembersPrefix),
				zap.Error(err),
			)
		} else {
			plog.Panicf("get storeRemovedMembers should never fail: %v", err)
		}
	}
	for _, n := range e.Node.Nodes {
		removed[MustParseMemberIDFromKey(n.Key)] = true
	}
	return members, removed
}

func clusterVersionFromStore(lg *zap.Logger, st v2store.Store) *semver.Version {
	e, err := st.Get(path.Join(storePrefix, "version"), false, false)
	if err != nil {
		if isKeyNotFound(err) {
			return nil
		}
		if lg != nil {
			lg.Panic(
				"failed to get cluster version from store",
				zap.String("path", path.Join(storePrefix, "version")),
				zap.Error(err),
			)
		} else {
			plog.Panicf("unexpected error (%v) when getting cluster version from store", err)
		}
	}
	return semver.Must(semver.NewVersion(*e.Node.Value))
}

// nodeToMember builds member from a key value node.
// the child nodes of the given node MUST be sorted by key.
func nodeToMember(n *v2store.NodeExtern) (*Member, error) {
	m := &Member{ID: MustParseMemberIDFromKey(n.Key)}
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

func MustParseMemberIDFromKey(key string) types.ID {
	id, err := types.IDFromString(path.Base(key))
	if err != nil {
		plog.Panicf("unexpected parse member id error: %v", err)
	}
	return id
}

func RemovedMemberStoreKey(id types.ID) string {
	return path.Join(storeRemovedMembersPrefix, id.String())
}
