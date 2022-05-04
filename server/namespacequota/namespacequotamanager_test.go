// Copyright 2021 The etcd Authors
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

package namespacequota

import (
	"bytes"
	"go.uber.org/zap/zaptest"
	"os"
	"reflect"
	"sort"
	"strconv"

	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.uber.org/zap"
	"io/ioutil"
	"path/filepath"
	"testing"
)

// TestSetNamespaceQuota Test the creation/updating of the namespace quota
func TestSetNamespaceQuota(t *testing.T) {
	lg := zap.NewNop()
	dir, be := NewTestBackend(t)
	defer os.RemoveAll(dir)
	defer be.Close()

	// Init a new namespace quota manager
	nqm := newNamespaceQuotaManager(lg, be, NamespaceQuotaManagerConfig{})
	nqm.Promote()
	nqm.SetReadView(newFakeReadView())

	// Test 1: Create a new quota
	// create a new quota
	sampleKey := []byte("foo")
	sampleQuota := &NamespaceQuota{
		Key:            sampleKey,
		QuotaByteCount: 3,
		QuotaKeyCount:  3,
	}

	// verify the quota doesn't exist yet
	nilQuota, err := nqm.GetNamespaceQuota(sampleKey)
	if err != ErrNamespaceQuotaNotFound && err != nil {
		t.Fatalf("quota failed to be fetched (%v)", err)
	}
	if nilQuota != nil {
		t.Fatalf("wanted nil, found (%v)", nilQuota)
	}

	// create the actual quota
	_, err = nqm.SetNamespaceQuota(sampleQuota.Key, sampleQuota.QuotaByteCount, sampleQuota.QuotaKeyCount)
	if err != nil {
		t.Fatalf("could not set quota 1 (%v)", err)
	}

	actualQuota, err := nqm.GetNamespaceQuota(sampleKey)
	if err != nil {
		t.Fatalf("could not get quota 1 (%v)", err)
	}

	// expected value
	expectedQuota := &NamespaceQuota{
		Key:            sampleQuota.Key,
		QuotaByteCount: sampleQuota.QuotaByteCount,
		QuotaKeyCount:  sampleQuota.QuotaKeyCount,
		UsageByteCount: 0, // 0 reflects the sum returned by the fakeRangeValueSize
		UsageKeyCount:  1, // 1 is to count the key itself
	}

	// verify each field
	if !bytes.Equal(actualQuota.Key, expectedQuota.Key) {
		t.Errorf("actual key = %s, expect key = %s", actualQuota.Key, expectedQuota.Key)
	}
	if actualQuota.QuotaByteCount != expectedQuota.QuotaByteCount {
		t.Errorf("quota byte count = %v, expect count %v", actualQuota.QuotaByteCount, expectedQuota.QuotaByteCount)
	}
	if actualQuota.QuotaKeyCount != expectedQuota.QuotaKeyCount {
		t.Errorf("quota key count = %v, expect count %v", actualQuota.QuotaKeyCount, expectedQuota.QuotaKeyCount)
	}
	if actualQuota.UsageByteCount != expectedQuota.UsageByteCount {
		t.Errorf("usage byte count = %v, expect usage %v", actualQuota.UsageByteCount, expectedQuota.UsageByteCount)
	}
	if actualQuota.UsageKeyCount != expectedQuota.UsageKeyCount {
		t.Errorf("usage key count = %v, expect usage %v", actualQuota.UsageKeyCount, expectedQuota.UsageKeyCount)
	}

	// Test 2: Update an already existing quota
	sampleQuota.QuotaByteCount = 5
	sampleQuota.QuotaKeyCount = 5

	// update the expectation
	expectedQuota.QuotaByteCount = sampleQuota.QuotaByteCount
	expectedQuota.QuotaKeyCount = sampleQuota.QuotaKeyCount

	_, err = nqm.SetNamespaceQuota(sampleQuota.Key, sampleQuota.QuotaByteCount, sampleQuota.QuotaKeyCount)
	if err != nil {
		t.Errorf("could not set quota 2 (%v)", err)
	}

	actualQuota, err = nqm.GetNamespaceQuota(sampleKey)
	if err != nil {
		t.Errorf("could not set quota 2 (%v)", err)
	}

	// verify each field
	if !bytes.Equal(actualQuota.Key, expectedQuota.Key) {
		t.Errorf("key = %v, expect key %v", actualQuota.Key, expectedQuota.Key)
	}
	if actualQuota.QuotaByteCount != expectedQuota.QuotaByteCount {
		t.Errorf("quota byte count = %v, expect count %v", actualQuota.QuotaByteCount, expectedQuota.QuotaByteCount)
	}
	if actualQuota.QuotaKeyCount != expectedQuota.QuotaKeyCount {
		t.Errorf("quota key count = %v, expect count %v", actualQuota.QuotaKeyCount, expectedQuota.QuotaKeyCount)
	}
	if actualQuota.UsageByteCount != expectedQuota.UsageByteCount {
		t.Errorf("usage byte count = %v, expect usage %v", actualQuota.UsageByteCount, expectedQuota.UsageByteCount)
	}
	if actualQuota.UsageKeyCount != expectedQuota.UsageKeyCount {
		t.Errorf("usage key count = %v, expect usage %v", actualQuota.UsageKeyCount, expectedQuota.UsageKeyCount)
	}
}

// TestGetNamespaceQuota Test the getting of the namespace quota
func TestGetNamespaceQuota(t *testing.T) {
	lg := zap.NewNop()
	dir, be := NewTestBackend(t)
	defer os.RemoveAll(dir)
	defer be.Close()

	// Init a new namespace quota manager
	nqm := newNamespaceQuotaManager(lg, be, NamespaceQuotaManagerConfig{})
	nqm.Promote()

	nqm.SetReadView(newFakeReadView())

	// Test 1: Create a new quota
	// create a new quota
	sampleKey := []byte("foo")
	sampleQuota := &NamespaceQuota{
		Key:            sampleKey,
		QuotaByteCount: 3,
		QuotaKeyCount:  3,
	}

	// create the actual quota
	_, err := nqm.SetNamespaceQuota(sampleQuota.Key, sampleQuota.QuotaByteCount, sampleQuota.QuotaKeyCount)
	if err != nil {
		t.Fatalf("could not set quota 1 (%v)", err)
	}

	actualQuota, err := nqm.GetNamespaceQuota(sampleKey)
	if err != nil {
		t.Fatalf("could not get quota 1 (%v)", err)
	}

	// expected value
	expectedQuota := &NamespaceQuota{
		Key:            sampleQuota.Key,
		QuotaByteCount: sampleQuota.QuotaByteCount,
		QuotaKeyCount:  sampleQuota.QuotaKeyCount,
		UsageByteCount: 0, // 0 reflects the sum returned by the fakeRangeValueSize
		UsageKeyCount:  1, // 1 is to count the key itself
	}

	// verify each field
	if !bytes.Equal(actualQuota.Key, expectedQuota.Key) {
		t.Errorf("key = %s, expect key %s", actualQuota.Key, expectedQuota.Key)
	}
	if actualQuota.QuotaByteCount != expectedQuota.QuotaByteCount {
		t.Errorf("quota byte count = %v, expect count %v", actualQuota.QuotaByteCount, expectedQuota.QuotaByteCount)
	}
	if actualQuota.QuotaKeyCount != expectedQuota.QuotaKeyCount {
		t.Errorf("quota key count = %v, expect count %v", actualQuota.QuotaKeyCount, expectedQuota.QuotaKeyCount)
	}
	if actualQuota.UsageByteCount != expectedQuota.UsageByteCount {
		t.Errorf("usage byte count = %v, expect usage %v", actualQuota.UsageByteCount, expectedQuota.UsageByteCount)
	}
	if actualQuota.UsageKeyCount != expectedQuota.UsageKeyCount {
		t.Errorf("usage key count = %v, expect usage %v", actualQuota.UsageKeyCount, expectedQuota.UsageKeyCount)
	}
}

func TestListNamespaceQuotas(t *testing.T) {
	// Define number of quotas
	n := 500
	lg := zap.NewNop()
	dir, be := NewTestBackend(t)
	defer os.RemoveAll(dir)
	defer be.Close()

	// Init a new namespace quota manager
	nqm := newNamespaceQuotaManager(lg, be, NamespaceQuotaManagerConfig{})
	nqm.Promote()

	nqm.SetReadView(newFakeReadView())
	var expectedList []*NamespaceQuota

	for i := 0; i < n; i++ {
		currQuota := &NamespaceQuota{
			Key:            []byte("foo" + strconv.Itoa(i)),
			QuotaByteCount: 5000,
			QuotaKeyCount:  5000,
		}
		nqm.namespaceQuotaTree.Put(currQuota)
		expectedList = append(expectedList, currQuota)
	}

	sort.Slice(expectedList, func(i, j int) bool {
		return bytes.Compare(expectedList[i].Key, expectedList[j].Key) < 0
	})

	// create the actual quota
	actualList := nqm.ListNamespaceQuotas()
	if !reflect.DeepEqual(actualList, expectedList) {
		t.Errorf("actual: %v, expected: %v", actualList, expectedList)
	}
}

func TestUpdateNamespaceQuotaUsage(t *testing.T) {
	lg := zap.NewNop()
	dir, be := NewTestBackend(t)
	defer os.RemoveAll(dir)
	defer be.Close()

	// Init a new namespace quota manager
	nqm := newNamespaceQuotaManager(lg, be, NamespaceQuotaManagerConfig{})
	nqm.Promote()

	nqm.SetReadView(newFakeReadView())

	// Test 1: Create a new quota
	// create a new quota
	sampleKey := []byte("foo")
	sampleQuota := &NamespaceQuota{
		Key:            sampleKey,
		QuotaByteCount: 5,
		QuotaKeyCount:  3,
	}
	// create the actual quota
	_, err := nqm.SetNamespaceQuota(sampleQuota.Key, sampleQuota.QuotaByteCount, sampleQuota.QuotaKeyCount)
	if err != nil {
		t.Fatalf("could not set quota 1 (%v)", err)
	}

	// create a new quota under sampleKey
	sampleKey2 := []byte("foo/1")
	sampleQuota = &NamespaceQuota{
		Key:            sampleKey2,
		QuotaByteCount: 4,
		QuotaKeyCount:  2,
	}
	// create the actual quota
	_, err = nqm.SetNamespaceQuota(sampleQuota.Key, sampleQuota.QuotaByteCount, sampleQuota.QuotaKeyCount)
	if err != nil {
		t.Fatalf("could not set quota 2 (%v)", err)
	}

	// create a new quota under sampleKey/1
	sampleKey3 := []byte("foo/1/2")
	sampleQuota = &NamespaceQuota{
		Key:            sampleKey3,
		QuotaByteCount: 4,
		QuotaKeyCount:  2,
	}
	// create the actual quota
	_, err = nqm.SetNamespaceQuota(sampleQuota.Key, sampleQuota.QuotaByteCount, sampleQuota.QuotaKeyCount)
	if err != nil {
		t.Fatalf("could not set quota 3 (%v)", err)
	}
}

// TestDeleteNamespaceQuota tests the deletion of given NamespaceQuota
func TestDeleteNamespaceQuota(t *testing.T) {
	lg := zap.NewNop()
	dir, be := NewTestBackend(t)
	defer os.RemoveAll(dir)
	defer be.Close()

	nqm := newNamespaceQuotaManager(lg, be, NamespaceQuotaManagerConfig{})
	nqm.Promote()
	nqm.SetReadView(newFakeReadView())

	// Quota is set to exact values of byte/key usages
	toBeDeleted, err := nqm.SetNamespaceQuota([]byte("foo"), 20, 3)
	if err != nil {
		t.Errorf("could not set namespace quota for %s", []byte("foo"))
	}
	_, err = nqm.SetNamespaceQuota([]byte("foo/1/1"), 8, 1)
	if err != nil {
		t.Errorf("could not set namespace quota for %s", []byte("foo/1/1"))
	}
	_, err = nqm.SetNamespaceQuota([]byte("foo/1/2"), 8, 1)
	if err != nil {
		t.Errorf("could not set namespace quota for %s", []byte("foo/1/2"))
	}

	// Delete a top level quota
	deleted, err := nqm.DeleteNamespaceQuota([]byte("foo"))
	if err != nil {
		t.Errorf("delete namespace quota failed for %s", []byte("foo"))
	}
	if !reflect.DeepEqual(deleted, toBeDeleted) {
		t.Errorf("deleted quota not equal to be quota to be deleted")
	}

	actual := nqm.ListNamespaceQuotas()

	expected := []*NamespaceQuota{{Key: []byte("foo/1/1"), QuotaByteCount: 8, UsageByteCount: 0, QuotaKeyCount: 1, UsageKeyCount: 1}, {Key: []byte("foo/1/2"), QuotaByteCount: 8, UsageByteCount: 0, QuotaKeyCount: 1, UsageKeyCount: 1}}

	if !reflect.DeepEqual(actual, expected) {
		for idx, value := range actual {
			t.Errorf("delete quota failed for %s, actual vs expected: \n%d vs %d\n%d vs %d\n%d vs %d\n%d vs %d", value.Key, value.QuotaByteCount, expected[idx].QuotaByteCount, value.QuotaKeyCount, expected[idx].QuotaKeyCount, value.UsageByteCount, expected[idx].UsageByteCount, value.UsageKeyCount, expected[idx].UsageKeyCount)
		}
	}

	// Delete a quota that doesn't exist
	nilValue, err := nqm.DeleteNamespaceQuota([]byte("nonexistantkey"))
	if err != ErrNamespaceQuotaNotFound || nilValue != nil {
		t.Errorf("unexpected error occured while deleting a non existant key")
	}
}

// TestDeleteRangeUpdateUsage tests the updates made when etcd performs a RangeDelete operation which encapsulates
// a subtree
func TestDeleteRangeUpdateUsage(t *testing.T) {
	lg := zap.NewNop()
	dir, be := NewTestBackend(t)
	defer os.RemoveAll(dir)
	defer be.Close()

	nqm := newNamespaceQuotaManager(lg, be, NamespaceQuotaManagerConfig{
		NamespaceQuotaEnforcement: 1,
	})
	nqm.Promote()
	nqm.SetReadView(newFakeReadViewDeleteRange())

	// The tree is assumed to have 3 keys & each corresponding value is worth 1 byte
	// foo (byte usage: 20 (4 + 8 + 8), key quota usage: 3)
	// foo/1/1 (byte usage: 8 (7 + 1), key usage: 1)
	// foo/1/2 (byte usage: 8 (7 + 1), key usage: 1)

	// Quota is set to exact values of byte/key usages
	nqm.SetNamespaceQuota([]byte("foo"), 20, 3)
	nqm.SetNamespaceQuota([]byte("foo/1/1"), 8, 1)
	nqm.SetNamespaceQuota([]byte("foo/1/2"), 8, 1)

	nqm.DeleteRangeUpdateUsage([][]byte{[]byte("foo/1/1"), []byte("foo/1/2")}, []int{1, 1})

	actual := nqm.ListNamespaceQuotas()

	currKey := "foo"
	// When foo/1/1 & foo/2/2 are removed, the count should be 20 - 8 - 8 = 4
	if actual[0].UsageByteCount != 4 {
		t.Errorf("key: %s, byte usage, expected: %d, actual: %d", currKey, 4, actual[0].UsageByteCount)
	}

	// Since the key is deleted and is a leaf key, should be 0
	currKey = "foo/1/1"
	if actual[1].UsageByteCount != 0 {
		t.Errorf("key: %s, usage, expected: %d, actual: %d", currKey, 0, actual[1].UsageByteCount)
	}
}

// TestNamespaceQuotaManagerRecovery ensures NamespaceQuotaManager recovers
// quota from persisted backend.
func TestNamespaceQuotaManagerRecovery(t *testing.T) {
	lg := zap.NewNop()
	dir, be := NewTestBackend(t)
	defer os.RemoveAll(dir)
	defer be.Close()

	nqm := newNamespaceQuotaManager(lg, be, NamespaceQuotaManagerConfig{})
	nqm.Promote()
	nqm.SetReadView(newFakeReadView())

	q1, _ := nqm.SetNamespaceQuota([]byte("foo/1"), 1, 1)
	q2, _ := nqm.SetNamespaceQuota([]byte("foo/2"), 1, 1)

	// Create a new NamespaceQuotaManager with the same backend
	nqmNew := newNamespaceQuotaManager(lg, be, NamespaceQuotaManagerConfig{})

	nq1, err := nqmNew.GetNamespaceQuota([]byte(q1.Key))
	if nq1 == nil || !reflect.DeepEqual(nq1, q1) || err != nil {
		t.Errorf("recover failed, quota unequal for %s", q1.Key)
	}

	nq2, err := nqmNew.GetNamespaceQuota([]byte(q2.Key))
	if nq2 == nil || !reflect.DeepEqual(nq2, q2) || err != nil {
		t.Errorf("recover failed, quota unequal %s", q2.Key)
	}
}

type fakeReadView struct {
	valueSizes []int
}

func newFakeReadView() *fakeReadView {
	fd := &fakeReadView{nil}
	return fd
}

func (frs *fakeReadView) RangeValueSize(key, end []byte) (keys [][]byte, valueSizes []int) {
	return [][]byte{[]byte("")}, []int{0}
}

func (frs *fakeReadView) GetValueSize(key []byte) (valueSize int, isFound bool) {
	return 0, true
}

type fakeReadViewDeleteRange struct {
	valueSizes []int
}

func newFakeReadViewDeleteRange() *fakeReadViewDeleteRange {
	fd := &fakeReadViewDeleteRange{nil}
	return fd
}

func (frs *fakeReadViewDeleteRange) RangeValueSize(key, end []byte) (keys [][]byte, valueSizes []int) {
	keys = [][]byte{[]byte("foo"), []byte("foo/1/1"), []byte("foo/1/2")}
	valueSizes = append(frs.valueSizes, 1, 1, 1)
	if bytes.Equal(key, []byte("foo")) {
		return keys, valueSizes
	} else if bytes.Equal(key, []byte("foo/1/1")) {
		return keys[1:2], valueSizes[1:2]
	} else if bytes.Equal(key, []byte("foo/1/2")) {
		return keys[2:], valueSizes[2:]
	} else if bytes.Equal(key, []byte("foo/1")) && bytes.Equal(end, []byte("foo/2")) {
		return keys[1:], valueSizes[1:]
	} else {
		return nil, nil
	}
}

func (frs *fakeReadViewDeleteRange) GetValueSize(key []byte) (valueSize int, isFound bool) {
	return 0, true
}

// NewTestBackend provides test backend
func NewTestBackend(t *testing.T) (string, backend.Backend) {
	tmpPath, err := ioutil.TempDir("", "namespacequota")
	lg := zaptest.NewLogger(t)
	if err != nil {
		t.Fatalf("failed to create tmpdir (%v)", err)
	}
	bcfg := backend.DefaultBackendConfig(lg)
	bcfg.Path = filepath.Join(tmpPath, "be")
	return tmpPath, backend.New(bcfg)
}
