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

package integration

import (
	"bytes"
	"context"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/namespacequota"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	"testing"
)

// TestV3NamespaceQuotaSet ensures the quota is created/updated correctly
func TestV3NamespaceQuotaSet(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, NamespaceQuotaEnforcement: 2})
	defer clus.Terminate(t)

	key := []byte("foo")
	var quotaByteCount uint64 = 10
	var quotaKeyCount uint64 = 10

	// create NamespaceQuota
	actual, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	expected := &namespacequota.NamespaceQuota{
		Key:            key,
		QuotaByteCount: quotaByteCount,
		UsageByteCount: 0,
		QuotaKeyCount:  quotaKeyCount,
		UsageKeyCount:  0,
	}

	if !bytes.Equal(actual.Quota.Key, expected.Key) {
		t.Errorf("key unequal")
	}
	if actual.Quota.QuotaByteCount != expected.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if actual.Quota.QuotaKeyCount != expected.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	if actual.Quota.UsageByteCount != expected.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if actual.Quota.UsageKeyCount != expected.UsageKeyCount {
		t.Errorf("key usage unequal")
	}
}

// TestV3NamespaceQuotaGet ensures the quota is fetched correctly
func TestV3NamespaceQuotaGet(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, NamespaceQuotaEnforcement: 2})
	defer clus.Terminate(t)

	key := []byte("foo")
	var quotaByteCount uint64 = 10
	var quotaKeyCount uint64 = 10

	// create NamespaceQuota
	expected, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	// get NamespaceQuota
	actual, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaGet(context.TODO(), &pb.NamespaceQuotaGetRequest{Key: key})
	if err != nil {
		t.Fatalf("could not get quota for key %s", key)
	}

	if !bytes.Equal(actual.Quota.Key, expected.Quota.Key) {
		t.Errorf("key unequal")
	}
	if actual.Quota.QuotaByteCount != expected.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if actual.Quota.QuotaKeyCount != expected.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	if actual.Quota.UsageByteCount != expected.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if actual.Quota.UsageKeyCount != expected.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}
}

// TestV3NamespaceQuotaDelete ensures the quota is deleted correctly
func TestV3NamespaceQuotaDelete(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, NamespaceQuotaEnforcement: 2})
	defer clus.Terminate(t)

	key := []byte("foo")
	var quotaByteCount uint64 = 10
	var quotaKeyCount uint64 = 10

	// create NamespaceQuota
	expected, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	// get NamespaceQuota
	actual, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaDelete(context.TODO(), &pb.NamespaceQuotaDeleteRequest{Key: key})
	if err != nil {
		t.Fatalf("could not delete quota for key %s", key)
	}

	// get NamespaceQuota
	_, err = integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaGet(context.TODO(), &pb.NamespaceQuotaGetRequest{Key: key})
	if err == nil && err != namespacequota.ErrNamespaceQuotaNotFound {
		t.Fatalf("quota not deleted correctly for key %s", key)
	}

	if !bytes.Equal(actual.Quota.Key, expected.Quota.Key) {
		t.Errorf("key unequal")
	}
	if actual.Quota.QuotaByteCount != expected.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if actual.Quota.QuotaKeyCount != expected.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	if actual.Quota.UsageByteCount != expected.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if actual.Quota.UsageKeyCount != expected.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}
}

// TestV3NamespaceQuotaUpdateEtcdPut ensures the quota consistency when etcd PUT operation is carried out
func TestV3NamespaceQuotaUpdateEtcdPut(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, NamespaceQuotaEnforcement: 2})
	defer clus.Terminate(t)

	key := []byte("foo")
	value := []byte("lessthan50bytesandkeyincluded")
	var quotaByteCount uint64 = 50
	var quotaKeyCount uint64 = 10

	// create a "foo" key
	_, err := integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: key, Value: value})
	if err != nil {
		t.Fatalf("could not create key %s", key)
	}

	// create NamespaceQuota
	expected, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	// verify quota was correctly set, and usage accounting
	if !bytes.Equal(key, expected.Quota.Key) {
		t.Errorf("key unequal")
	}
	if quotaByteCount != expected.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if quotaKeyCount != expected.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	if uint64(len(key)+len(value)) != expected.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if 1 != expected.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}

	_, err = integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaGet(context.TODO(), &pb.NamespaceQuotaGetRequest{Key: key})
	if err != nil {
		t.Errorf("could not get quota for key %s", key)
	}

	// attempt to create a "foo/1" key, it should be rejected
	exceededKey := []byte("foo/1")
	exceededValue := []byte("morethan50byteswithkeyincluded")
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: exceededKey, Value: exceededValue})

	if err != nil && err.Error() != rpctypes.ErrGRPCNamespaceQuotaExceeded.Error() {
		t.Errorf("quota not exceeded but was expected to be exceeded: %v", err)
	}
}

// TestV3NamespaceQuotaUpdateEtcdDelete ensures the quota consistency when etcd DELETE operation is carried out
func TestV3NamespaceQuotaUpdateEtcdDelete(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, NamespaceQuotaEnforcement: 2})
	defer clus.Terminate(t)

	key := []byte("foo")
	value := []byte("lessthan50bytesandkeyincluded")
	var quotaByteCount uint64 = 50
	var quotaKeyCount uint64 = 10

	// create a "foo" key
	_, err := integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: key, Value: value})
	if err != nil {
		t.Fatalf("could not create key %s", key)
	}

	// create NamespaceQuota
	expected, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	// verify quota was correctly set, and usage accounting
	if !bytes.Equal(key, expected.Quota.Key) {
		t.Errorf("key unequal")
	}
	if quotaByteCount != expected.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if quotaKeyCount != expected.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	if uint64(len(key)+len(value)) != expected.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if 1 != expected.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}

	_, err = integration.ToGRPC(clus.RandClient()).KV.DeleteRange(context.TODO(), &pb.DeleteRangeRequest{Key: key, RangeEnd: nil})
	if err != nil {
		t.Fatalf("could not delete key %s", key)
	}

	getQuota, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaGet(context.TODO(), &pb.NamespaceQuotaGetRequest{Key: key})
	if err != nil {
		t.Errorf("could not get quota for key %s", key)
	}

	// verify quota was correctly set, and usage accounting
	if !bytes.Equal(key, getQuota.Quota.Key) {
		t.Errorf("key unequal")
	}
	if quotaByteCount != getQuota.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if quotaKeyCount != getQuota.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	if 0 != getQuota.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if 0 != getQuota.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}

	// attempt to create a "foo/1" key, it should be accepted, since the quota is not exceeded anymore
	exceededKey := []byte("foo/1")
	exceededValue := []byte("afairlybigvaluetobeadded")
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: exceededKey, Value: exceededValue})

	if err != nil {
		t.Error("quota not exceeded but was expected to be exceeded")
	}
}

// TestV3NamespaceQuotaSetQuotaFirst ensures the quota consistency when etcd key doesn't exist yet
func TestV3NamespaceQuotaSetQuotaFirst(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, NamespaceQuotaEnforcement: 2})
	defer clus.Terminate(t)

	key := []byte("foo")
	value := []byte("lessthan50bytesandkeyincluded")
	var quotaByteCount uint64 = 50
	var quotaKeyCount uint64 = 10

	// create NamespaceQuota
	_, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	// create a "foo" key
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: key, Value: value})
	if err != nil {
		t.Fatalf("could not create key %s", key)
	}
	expected, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaGet(context.TODO(), &pb.NamespaceQuotaGetRequest{Key: key})
	if err != nil {
		t.Errorf("could not get quota for key %s", key)
	}

	// verify quota was correctly set, and usage accounting
	if !bytes.Equal(key, expected.Quota.Key) {
		t.Errorf("key unequal")
	}
	if quotaByteCount != expected.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if quotaKeyCount != expected.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	total := uint64(len(key) + len(value))
	if total != expected.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if 1 != expected.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}

	// attempt to create a "foo/1" key, it should be rejected
	exceededKey := []byte("foo/1")
	exceededValue := []byte("morethan50byteswithkeyincluded")
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: exceededKey, Value: exceededValue})

	if err != nil && err.Error() != rpctypes.ErrGRPCNamespaceQuotaExceeded.Error() {
		t.Error("quota not exceeded but was expected to be exceeded")
	}
}

// TestV3NamespaceQuotaEnforcementHardMode ensures the quota enforcement in hard mode denies requests over quota
func TestV3NamespaceQuotaEnforcementHardMode(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, NamespaceQuotaEnforcement: 2})
	defer clus.Terminate(t)

	key := []byte("foo")
	value := []byte("lessthan50bytesandkeyincluded")
	var quotaByteCount uint64 = 50
	var quotaKeyCount uint64 = 10

	// create NamespaceQuota
	_, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	// create a "foo" key
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: key, Value: value})
	if err != nil {
		t.Fatalf("could not create key %s", key)
	}

	expected, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaGet(context.TODO(), &pb.NamespaceQuotaGetRequest{Key: key})
	if err != nil {
		t.Errorf("could not get quota for key %s", key)
	}

	// verify quota was correctly set, and usage accounting
	if !bytes.Equal(key, expected.Quota.Key) {
		t.Errorf("key unequal")
	}
	if quotaByteCount != expected.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if quotaKeyCount != expected.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	total := uint64(len(key) + len(value))
	if total != expected.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if 1 != expected.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}

	// attempt to create a "foo/1" key, it should be rejected
	exceededKey := []byte("foo/1")
	exceededValue := []byte("morethan50byteswithkeyincluded")
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: exceededKey, Value: exceededValue})

	if err != nil && err.Error() != rpctypes.ErrGRPCNamespaceQuotaExceeded.Error() {
		t.Error("quota not exceeded but was expected to be exceeded")
	}
}

// TestV3NamespaceQuotaEnforcementSoftMode ensures the quota enforcement in soft mode denies requests over quota
func TestV3NamespaceQuotaEnforcementSoftMode(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, NamespaceQuotaEnforcement: 1})
	defer clus.Terminate(t)

	key := []byte("foo")
	value := []byte("lessthan50bytesandkeyincluded")
	var quotaByteCount uint64 = 50
	var quotaKeyCount uint64 = 10

	// create NamespaceQuota
	_, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	// create a "foo" key
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: key, Value: value})
	if err != nil {
		t.Fatalf("could not create key %s", key)
	}

	expected, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaGet(context.TODO(), &pb.NamespaceQuotaGetRequest{Key: key})
	if err != nil {
		t.Errorf("could not get quota for key %s", key)
	}

	// verify quota was correctly set, and usage accounting
	if !bytes.Equal(key, expected.Quota.Key) {
		t.Errorf("key unequal")
	}
	if quotaByteCount != expected.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if quotaKeyCount != expected.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	total := uint64(len(key) + len(value))
	if total != expected.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	if 1 != expected.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}

	// attempt to create a "foo/1" key, it should be accepted due to soft mode enforcement
	exceededKey := []byte("foo/1")
	exceededValue := []byte("morethan50byteswithkeyincluded")
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: exceededKey, Value: exceededValue})

	if err != nil {
		t.Errorf("quota exceeded but was expected to be not exceeded, %v", err)
	}
}

// TestV3NamespaceQuotaEnforcementDisabled ensures the quota enforcement in disabled accepts requests over quota and does
// not record any quota operations
func TestV3NamespaceQuotaEnforcementDisabled(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	key := []byte("foo")
	value := []byte("lessthan50bytesandkeyincluded")
	var quotaByteCount uint64 = 50
	var quotaKeyCount uint64 = 10

	// create NamespaceQuota
	_, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s", key)
	}

	// create a "foo" key
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: key, Value: value})
	if err != nil {
		t.Fatalf("could not create key %s", key)
	}

	expected, err := integration.ToGRPC(clus.RandClient()).NamespaceQuota.NamespaceQuotaGet(context.TODO(), &pb.NamespaceQuotaGetRequest{Key: key})
	if err != nil {
		t.Errorf("could not get quota for key %s", key)
	}

	// verify quota was correctly set, and usage accounting
	if !bytes.Equal(key, expected.Quota.Key) {
		t.Errorf("key unequal")
	}
	if quotaByteCount != expected.Quota.QuotaByteCount {
		t.Errorf("byte quota unequal")
	}
	if quotaKeyCount != expected.Quota.QuotaKeyCount {
		t.Errorf("key quota unequal")
	}
	// disabling quotas will stop from recording any operations
	if 0 != expected.Quota.UsageByteCount {
		t.Errorf("byte usage unequal")
	}
	// disabling quotas will stop from recording any operations
	if 0 != expected.Quota.UsageKeyCount {
		t.Errorf("key usage unequal")
	}

	// attempt to create a "foo/1" key, it should be accepted even when the quota is set
	// quota enforcement is disabled
	exceededKey := []byte("foo/1")
	exceededValue := []byte("morethan50byteswithkeyincluded")
	_, err = integration.ToGRPC(clus.RandClient()).KV.Put(context.TODO(), &pb.PutRequest{Key: exceededKey, Value: exceededValue})
	if err != nil {
		t.Error("quota exceeded but was expected to be not exceeded")
	}
}

// TestV3NamespaceQuotaAuthEnabledOperations tests the namespace quota set/delete/list operations when auth is enabled
// with admin/root user
func TestV3NamespaceQuotaAuthEnabledOperations(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	key := []byte("foo")
	var quotaByteCount uint64 = 50
	var quotaKeyCount uint64 = 10

	// setup non-root user
	users := []user{
		{
			name:     "user1",
			password: "user1-123",
			role:     "role1",
			key:      string(key),
		},
	}
	authSetupUsers(t, integration.ToGRPC(clus.Client(0)).Auth, users)

	// setup root user
	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	rootc, cerr := clientv3.New(clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "root", Password: "123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer rootc.Close()

	userc, cerr := clientv3.New(clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "user1-123"})
	if cerr != nil {
		t.Fatal(cerr)
	}
	defer userc.Close()

	// Following operations are tested, first with a normal user with READONLY permissions which should return an error
	// regarding insufficient permissions and then a "root" user should pass
	// 1. Set NamespaceQuota
	// 2. List NamespaceQuota
	// 3. Delete NamespaceQuota

	// Set NamespaceQuota
	_, err := integration.ToGRPC(userc).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err == nil {
		if err != rpctypes.ErrPermissionDenied {
			t.Fatalf("unexpected error while setting quota: %v", err)
		}
		t.Fatalf("should be unable to set quota for key %s", key)
	}
	_, err = integration.ToGRPC(rootc).NamespaceQuota.NamespaceQuotaSet(context.TODO(), &pb.NamespaceQuotaSetRequest{Key: key, QuotaByteCount: quotaByteCount, QuotaKeyCount: quotaKeyCount})
	if err != nil {
		t.Fatalf("could not set quota for key %s, %v", key, err)
	}

	// List NamespaceQuota
	_, err = integration.ToGRPC(userc).NamespaceQuota.NamespaceQuotaList(context.TODO(), &pb.NamespaceQuotaListRequest{})
	if err == nil {
		if err != rpctypes.ErrPermissionDenied {
			t.Fatalf("unexpected error while listing quota: %v", err)
		}
		t.Fatalf("should be unable to list quota")
	}
	_, err = integration.ToGRPC(rootc).NamespaceQuota.NamespaceQuotaList(context.TODO(), &pb.NamespaceQuotaListRequest{})
	if err != nil {
		t.Fatalf("could not list quota, %v", err)
	}

	// Delete NamespaceQuota
	_, err = integration.ToGRPC(userc).NamespaceQuota.NamespaceQuotaDelete(context.TODO(), &pb.NamespaceQuotaDeleteRequest{Key: key})
	if err == nil {
		if err != rpctypes.ErrPermissionDenied {
			t.Fatalf("unexpected error while deleting quota: %v", err)
		}
		t.Fatalf("should be unable to delete quota for key %s", key)
	}
	_, err = integration.ToGRPC(rootc).NamespaceQuota.NamespaceQuotaDelete(context.TODO(), &pb.NamespaceQuotaDeleteRequest{Key: key})
	if err != nil {
		t.Fatalf("could not delete quota for key %s, %v", key, err)
	}
}
