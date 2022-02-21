package v3discovery

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/client/v3"

	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"
)

// fakeKVForClusterSize is used to test getClusterSize.
type fakeKVForClusterSize struct {
	*fakeBaseKV
	clusterSizeStr string
}

// We only need to overwrite the method `Get`.
func (fkv *fakeKVForClusterSize) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if fkv.clusterSizeStr == "" {
		// cluster size isn't configured in this case.
		return &clientv3.GetResponse{}, nil
	}

	return &clientv3.GetResponse{
		Kvs: []*mvccpb.KeyValue{
			{
				Value: []byte(fkv.clusterSizeStr),
			},
		},
	}, nil
}

func TestGetClusterSize(t *testing.T) {
	cases := []struct {
		name           string
		clusterSizeStr string
		expectedErr    error
		expectedSize   int
	}{
		{
			name:           "cluster size not defined",
			clusterSizeStr: "",
			expectedErr:    ErrSizeNotFound,
		},
		{
			name:           "invalid cluster size",
			clusterSizeStr: "invalidSize",
			expectedErr:    ErrBadSizeKey,
		},
		{
			name:           "valid cluster size",
			clusterSizeStr: "3",
			expectedErr:    nil,
			expectedSize:   3,
		},
	}

	lg, err := zap.NewProduction()
	if err != nil {
		t.Errorf("Failed to create a logger, error: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &discovery{
				lg: lg,
				c: &clientv3.Client{
					KV: &fakeKVForClusterSize{
						fakeBaseKV:     &fakeBaseKV{},
						clusterSizeStr: tc.clusterSizeStr,
					},
				},
				cfg:          &DiscoveryConfig{},
				clusterToken: "fakeToken",
			}

			if cs, err := d.getClusterSize(); err != tc.expectedErr {
				t.Errorf("Unexpected error, expected: %v got: %v", tc.expectedErr, err)
			} else {
				if err == nil && cs != tc.expectedSize {
					t.Errorf("Unexpected cluster size, expected: %d got: %d", tc.expectedSize, cs)
				}
			}
		})
	}
}

// fakeKVForClusterMembers is used to test getClusterMembers.
type fakeKVForClusterMembers struct {
	*fakeBaseKV
	members []memberInfo
}

// We only need to overwrite method `Get`.
func (fkv *fakeKVForClusterMembers) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	kvs := memberInfoToKeyValues(fkv.members)

	return &clientv3.GetResponse{
		Header: &etcdserverpb.ResponseHeader{
			Revision: 10,
		},
		Kvs: kvs,
	}, nil
}

func memberInfoToKeyValues(members []memberInfo) []*mvccpb.KeyValue {
	kvs := make([]*mvccpb.KeyValue, 0)
	for _, mi := range members {
		kvs = append(kvs, &mvccpb.KeyValue{
			Key:            []byte(mi.peerRegKey),
			Value:          []byte(mi.peerURLsMap),
			CreateRevision: mi.createRev,
		})
	}

	return kvs
}

func TestGetClusterMembers(t *testing.T) {
	actualMemberInfo := []memberInfo{
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   8,
		},
		{
			// invalid peer registry key
			peerRegKey:  "/invalidPrefix/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			// invalid peer info format
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(103).String(),
			peerURLsMap: "infra3=http://192.168.0.103:2380",
			createRev:   7,
		},
		{
			// duplicate peer
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   2,
		},
	}

	// sort by CreateRevision
	expectedMemberInfo := []memberInfo{
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(103).String(),
			peerURLsMap: "infra3=http://192.168.0.103:2380",
			createRev:   7,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   8,
		},
	}

	lg, err := zap.NewProduction()
	if err != nil {
		t.Errorf("Failed to create a logger, error: %v", err)
	}

	d := &discovery{
		lg: lg,
		c: &clientv3.Client{
			KV: &fakeKVForClusterMembers{
				fakeBaseKV: &fakeBaseKV{},
				members:    actualMemberInfo,
			},
		},
		cfg:          &DiscoveryConfig{},
		clusterToken: "fakeToken",
	}

	clsInfo, _, err := d.getClusterMembers()
	if err != nil {
		t.Errorf("Failed to get cluster members, error: %v", err)
	}

	if clsInfo.Len() != len(expectedMemberInfo) {
		t.Errorf("unexpected member count, expected: %d, got: %d", len(expectedMemberInfo), clsInfo.Len())
	}

	for i, m := range clsInfo.members {
		if m != expectedMemberInfo[i] {
			t.Errorf("unexpected member[%d], expected: %v, got: %v", i, expectedMemberInfo[i], m)
		}
	}
}

// fakeKVForCheckCluster is used to test checkCluster.
type fakeKVForCheckCluster struct {
	*fakeBaseKV
	t                 *testing.T
	token             string
	clusterSizeStr    string
	members           []memberInfo
	getSizeRetries    int
	getMembersRetries int
}

// We only need to overwrite method `Get`.
func (fkv *fakeKVForCheckCluster) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	clusterSizeKey := fmt.Sprintf("/_etcd/registry/%s/_config/size", fkv.token)
	clusterMembersKey := fmt.Sprintf("/_etcd/registry/%s/members", fkv.token)

	if key == clusterSizeKey {
		if fkv.getSizeRetries > 0 {
			fkv.getSizeRetries--
			// discovery client should retry on error.
			return nil, errors.New("get cluster size failed")
		}
		return &clientv3.GetResponse{
			Kvs: []*mvccpb.KeyValue{
				{
					Value: []byte(fkv.clusterSizeStr),
				},
			},
		}, nil

	} else if key == clusterMembersKey {
		if fkv.getMembersRetries > 0 {
			fkv.getMembersRetries--
			// discovery client should retry on error.
			return nil, errors.New("get cluster members failed")
		}
		kvs := memberInfoToKeyValues(fkv.members)

		return &clientv3.GetResponse{
			Header: &etcdserverpb.ResponseHeader{
				Revision: 10,
			},
			Kvs: kvs,
		}, nil
	} else {
		fkv.t.Errorf("unexpected key: %s", key)
		return nil, fmt.Errorf("unexpected key: %s", key)
	}
}

func TestCheckCluster(t *testing.T) {
	actualMemberInfo := []memberInfo{
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   8,
		},
		{
			// invalid peer registry key
			peerRegKey:  "/invalidPrefix/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			// invalid peer info format
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(103).String(),
			peerURLsMap: "infra3=http://192.168.0.103:2380",
			createRev:   7,
		},
		{
			// duplicate peer
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   2,
		},
	}

	// sort by CreateRevision
	expectedMemberInfo := []memberInfo{
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(103).String(),
			peerURLsMap: "infra3=http://192.168.0.103:2380",
			createRev:   7,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   8,
		},
	}

	cases := []struct {
		name              string
		memberId          types.ID
		getSizeRetries    int
		getMembersRetries int
		expectedError     error
	}{
		{
			name:              "no retries",
			memberId:          101,
			getSizeRetries:    0,
			getMembersRetries: 0,
			expectedError:     nil,
		},
		{
			name:              "2 retries for getClusterSize",
			memberId:          102,
			getSizeRetries:    2,
			getMembersRetries: 0,
			expectedError:     nil,
		},
		{
			name:              "2 retries for getClusterMembers",
			memberId:          103,
			getSizeRetries:    0,
			getMembersRetries: 2,
			expectedError:     nil,
		},
		{
			name:              "error due to cluster full",
			memberId:          104,
			getSizeRetries:    0,
			getMembersRetries: 0,
			expectedError:     ErrFullCluster,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lg, err := zap.NewProduction()
			if err != nil {
				t.Errorf("Failed to create a logger, error: %v", err)
			}

			fkv := &fakeKVForCheckCluster{
				fakeBaseKV:        &fakeBaseKV{},
				t:                 t,
				token:             "fakeToken",
				clusterSizeStr:    "3",
				members:           actualMemberInfo,
				getSizeRetries:    tc.getSizeRetries,
				getMembersRetries: tc.getMembersRetries,
			}

			d := &discovery{
				lg: lg,
				c: &clientv3.Client{
					KV: fkv,
				},
				cfg:          &DiscoveryConfig{},
				clusterToken: "fakeToken",
				memberId:     tc.memberId,
				clock:        clockwork.NewRealClock(),
			}

			clsInfo, _, _, err := d.checkCluster()
			if err != tc.expectedError {
				t.Errorf("Unexpected error, expected: %v, got: %v", tc.expectedError, err)
			}

			if err == nil {
				if fkv.getSizeRetries != 0 || fkv.getMembersRetries != 0 {
					t.Errorf("Discovery client did not retry checking cluster on error, remaining etries: (%d, %d)", fkv.getSizeRetries, fkv.getMembersRetries)
				}

				if clsInfo.Len() != len(expectedMemberInfo) {
					t.Errorf("Unexpected member count, expected: %d, got: %d", len(expectedMemberInfo), clsInfo.Len())
				}

				for mIdx, m := range clsInfo.members {
					if m != expectedMemberInfo[mIdx] {
						t.Errorf("Unexpected member[%d], expected: %v, got: %v", mIdx, expectedMemberInfo[mIdx], m)
					}
				}
			}
		})
	}
}

// fakeKVForRegisterSelf is used to test registerSelf.
type fakeKVForRegisterSelf struct {
	*fakeBaseKV
	t                *testing.T
	expectedRegKey   string
	expectedRegValue string
	retries          int
}

// We only need to overwrite method `Put`.
func (fkv *fakeKVForRegisterSelf) Put(ctx context.Context, key string, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	if key != fkv.expectedRegKey {
		fkv.t.Errorf("unexpected register key, expected: %s, got: %s", fkv.expectedRegKey, key)
	}

	if val != fkv.expectedRegValue {
		fkv.t.Errorf("unexpected register value, expected: %s, got: %s", fkv.expectedRegValue, val)
	}

	if fkv.retries > 0 {
		fkv.retries--
		// discovery client should retry on error.
		return nil, errors.New("register self failed")
	}

	return nil, nil
}

func TestRegisterSelf(t *testing.T) {
	cases := []struct {
		name             string
		token            string
		memberId         types.ID
		expectedRegKey   string
		expectedRegValue string
		retries          int // when retries > 0, then return an error on Put request.
	}{
		{
			name:             "no retry with token1",
			token:            "token1",
			memberId:         101,
			expectedRegKey:   "/_etcd/registry/token1/members/" + types.ID(101).String(),
			expectedRegValue: "infra=http://127.0.0.1:2380",
			retries:          0,
		},
		{
			name:             "no retry with token2",
			token:            "token2",
			memberId:         102,
			expectedRegKey:   "/_etcd/registry/token2/members/" + types.ID(102).String(),
			expectedRegValue: "infra=http://127.0.0.1:2380",
			retries:          0,
		},
		{
			name:             "2 retries",
			token:            "token3",
			memberId:         103,
			expectedRegKey:   "/_etcd/registry/token3/members/" + types.ID(103).String(),
			expectedRegValue: "infra=http://127.0.0.1:2380",
			retries:          2,
		},
	}

	lg, err := zap.NewProduction()
	if err != nil {
		t.Errorf("Failed to create a logger, error: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fkv := &fakeKVForRegisterSelf{
				fakeBaseKV:       &fakeBaseKV{},
				t:                t,
				expectedRegKey:   tc.expectedRegKey,
				expectedRegValue: tc.expectedRegValue,
				retries:          tc.retries,
			}

			d := &discovery{
				lg:           lg,
				clusterToken: tc.token,
				memberId:     tc.memberId,
				cfg:          &DiscoveryConfig{},
				c: &clientv3.Client{
					KV: fkv,
				},
				clock: clockwork.NewRealClock(),
			}

			if err := d.registerSelf(tc.expectedRegValue); err != nil {
				t.Errorf("Error occuring on register member self: %v", err)
			}

			if fkv.retries != 0 {
				t.Errorf("Discovery client did not retry registering itself on error, remaining retries: %d", fkv.retries)
			}
		})
	}
}

// fakeWatcherForWaitPeers is used to test waitPeers.
type fakeWatcherForWaitPeers struct {
	*fakeBaseWatcher
	t       *testing.T
	token   string
	members []memberInfo
}

// We only need to overwrite method `Watch`.
func (fw *fakeWatcherForWaitPeers) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	expectedWatchKey := fmt.Sprintf("/_etcd/registry/%s/members", fw.token)
	if key != expectedWatchKey {
		fw.t.Errorf("unexpected watch key, expected: %s, got: %s", expectedWatchKey, key)
	}

	ch := make(chan clientv3.WatchResponse, 1)
	go func() {
		for _, mi := range fw.members {
			ch <- clientv3.WatchResponse{
				Events: []*clientv3.Event{
					{
						Kv: &mvccpb.KeyValue{
							Key:            []byte(mi.peerRegKey),
							Value:          []byte(mi.peerURLsMap),
							CreateRevision: mi.createRev,
						},
					},
				},
			}
		}
		close(ch)
	}()
	return ch
}

func TestWaitPeers(t *testing.T) {
	actualMemberInfo := []memberInfo{
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   8,
		},
		{
			// invalid peer registry key
			peerRegKey:  "/invalidPrefix/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			// invalid peer info format
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(103).String(),
			peerURLsMap: "infra3=http://192.168.0.103:2380",
			createRev:   7,
		},
		{
			// duplicate peer
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   2,
		},
	}

	// sort by CreateRevision
	expectedMemberInfo := []memberInfo{
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(102).String(),
			peerURLsMap: "infra2=http://192.168.0.102:2380",
			createRev:   6,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(103).String(),
			peerURLsMap: "infra3=http://192.168.0.103:2380",
			createRev:   7,
		},
		{
			peerRegKey:  "/_etcd/registry/fakeToken/members/" + types.ID(101).String(),
			peerURLsMap: "infra1=http://192.168.0.100:2380",
			createRev:   8,
		},
	}

	lg, err := zap.NewProduction()
	if err != nil {
		t.Errorf("Failed to create a logger, error: %v", err)
	}

	d := &discovery{
		lg: lg,
		c: &clientv3.Client{
			KV: &fakeBaseKV{},
			Watcher: &fakeWatcherForWaitPeers{
				fakeBaseWatcher: &fakeBaseWatcher{},
				t:               t,
				token:           "fakeToken",
				members:         actualMemberInfo,
			},
		},
		cfg:          &DiscoveryConfig{},
		clusterToken: "fakeToken",
	}

	cls := clusterInfo{
		clusterToken: "fakeToken",
	}

	d.waitPeers(&cls, 3, 0)

	if cls.Len() != len(expectedMemberInfo) {
		t.Errorf("unexpected member number returned by watch, expected: %d, got: %d", len(expectedMemberInfo), cls.Len())
	}

	for i, m := range cls.members {
		if m != expectedMemberInfo[i] {
			t.Errorf("unexpected member[%d] returned by watch, expected: %v, got: %v", i, expectedMemberInfo[i], m)
		}
	}
}

func TestGetInitClusterStr(t *testing.T) {
	cases := []struct {
		name           string
		members        []memberInfo
		clusterSize    int
		expectedResult string
		expectedError  error
	}{
		{
			name: "1 member",
			members: []memberInfo{
				{
					peerURLsMap: "infra2=http://192.168.0.102:2380",
				},
			},
			clusterSize:    1,
			expectedResult: "infra2=http://192.168.0.102:2380",
			expectedError:  nil,
		},
		{
			name: "2 members",
			members: []memberInfo{
				{
					peerURLsMap: "infra2=http://192.168.0.102:2380",
				},
				{
					peerURLsMap: "infra3=http://192.168.0.103:2380",
				},
			},
			clusterSize:    2,
			expectedResult: "infra2=http://192.168.0.102:2380,infra3=http://192.168.0.103:2380",
			expectedError:  nil,
		},
		{
			name: "3 members",
			members: []memberInfo{
				{
					peerURLsMap: "infra2=http://192.168.0.102:2380",
				},
				{
					peerURLsMap: "infra3=http://192.168.0.103:2380",
				},
				{
					peerURLsMap: "infra1=http://192.168.0.100:2380",
				},
			},
			clusterSize:    3,
			expectedResult: "infra2=http://192.168.0.102:2380,infra3=http://192.168.0.103:2380,infra1=http://192.168.0.100:2380",
			expectedError:  nil,
		},
		{
			name: "should ignore redundant member",
			members: []memberInfo{
				{
					peerURLsMap: "infra2=http://192.168.0.102:2380",
				},
				{
					peerURLsMap: "infra3=http://192.168.0.103:2380",
				},
				{
					peerURLsMap: "infra1=http://192.168.0.100:2380",
				},
				{
					peerURLsMap: "infra4=http://192.168.0.104:2380",
				},
			},
			clusterSize:    3,
			expectedResult: "infra2=http://192.168.0.102:2380,infra3=http://192.168.0.103:2380,infra1=http://192.168.0.100:2380",
			expectedError:  nil,
		},
		{
			name: "invalid_peer_url",
			members: []memberInfo{
				{
					peerURLsMap: "infra2=http://192.168.0.102:2380",
				},
				{
					peerURLsMap: "infra3=http://192.168.0.103", //not host:port
				},
			},
			clusterSize:    2,
			expectedResult: "infra2=http://192.168.0.102:2380,infra3=http://192.168.0.103:2380",
			expectedError:  ErrInvalidURL,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clsInfo := &clusterInfo{
				members: tc.members,
			}

			retStr, err := clsInfo.getInitClusterStr(tc.clusterSize)
			if err != tc.expectedError {
				t.Errorf("Unexpected error, expected: %v, got: %v", tc.expectedError, err)
			}

			if err == nil {
				if retStr != tc.expectedResult {
					t.Errorf("Unexpected result, expected: %s, got: %s", tc.expectedResult, retStr)
				}
			}
		})
	}
}

// fakeBaseKV is the base struct implementing the interface `clientv3.KV`.
type fakeBaseKV struct{}

func (fkv *fakeBaseKV) Put(ctx context.Context, key string, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return nil, nil
}

func (fkv *fakeBaseKV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return nil, nil
}

func (fkv *fakeBaseKV) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return nil, nil
}

func (fkv *fakeBaseKV) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	return nil, nil
}

func (fkv *fakeBaseKV) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return clientv3.OpResponse{}, nil
}

func (fkv *fakeBaseKV) Txn(ctx context.Context) clientv3.Txn {
	return nil
}

// fakeBaseWatcher is the base struct implementing the interface `clientv3.Watcher`.
type fakeBaseWatcher struct{}

func (fw *fakeBaseWatcher) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	return nil
}

func (fw *fakeBaseWatcher) RequestProgress(ctx context.Context) error {
	return nil
}

func (fw *fakeBaseWatcher) Close() error {
	return nil
}
