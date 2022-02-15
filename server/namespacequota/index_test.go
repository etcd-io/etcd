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
	"go.uber.org/zap"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

// TestIndexGet tests the get function to fetch a key from the quotaIndexTree
func TestIndexGet(t *testing.T) {
	ti := NewQuotaTreeIndex(zap.NewExample())

	key1 := []byte("foo1")
	foo1 := &NamespaceQuota{
		Key:            key1,
		QuotaByteCount: 4,
		QuotaKeyCount:  3,
	}

	key2 := []byte("foo2")
	foo2 := &NamespaceQuota{
		Key:            key2,
		QuotaByteCount: 3,
		QuotaKeyCount:  4,
	}

	_, status := ti.Put(foo1)
	if status {
		t.Errorf("error while setting quota #1")
	}

	_, status = ti.Put(foo2)
	if status {
		t.Errorf("error while setting quota #2")
	}

	q := ti.Get([]byte(key1))
	if !reflect.DeepEqual(q, foo1) {
		t.Errorf("quota unequal, got %v want %v", q, foo1)
	}

	q = ti.Get([]byte(key2))
	if !reflect.DeepEqual(q, foo2) {
		t.Errorf("quota unequal, got %v want %v", q, foo2)
	}
}

func TestIndexGetAllNamespaceQuotas(t *testing.T) {
	n := 100
	ti := NewQuotaTreeIndex(zap.NewExample())
	var expectedList []*NamespaceQuota

	for i := 0; i < n; i++ {
		currQuota := &NamespaceQuota{
			Key:            []byte("foo" + strconv.Itoa(i)),
			QuotaByteCount: 5000,
			QuotaKeyCount:  5000,
		}
		ti.Put(currQuota)
		expectedList = append(expectedList, currQuota)
	}

	sort.Slice(expectedList, func(i, j int) bool {
		return bytes.Compare(expectedList[i].Key, expectedList[j].Key) < 0
	})

	// create the actual quota
	actualList := ti.GetAllNamespaceQuotas()

	if len(expectedList) != len(actualList) {
		t.Errorf("expected %d, got %d", len(expectedList), len(actualList))
	}

	if !reflect.DeepEqual(actualList, expectedList) {
		for i, actual := range actualList {
			if !bytes.Equal(actual.Key, expectedList[i].Key) {
				t.Errorf("key unequal %s vs %s", actual.Key, expectedList[i].Key)
			}
			if actual.QuotaByteCount != expectedList[i].QuotaByteCount {
				t.Errorf("byte quota unequal")
			}
			if actual.QuotaKeyCount != expectedList[i].QuotaKeyCount {
				t.Errorf("key quota unequal")
			}
			if actual.UsageByteCount != expectedList[i].UsageByteCount {
				t.Errorf("byte usage unequal")
			}
			if actual.UsageKeyCount != expectedList[i].UsageKeyCount {
				t.Errorf("key usage unequal")
			}
		}
		t.Errorf("actual: %v, expected: %v", actualList, expectedList)
	}
}

// TestKeyIndexPut test key index put tests if the put
// operation was successfully and correctly performed
func TestKeyIndexPut(t *testing.T) {
	testKey := "foo"

	// Create a sample quota
	testNamespaceQuota := &NamespaceQuota{
		Key:            []byte(testKey),
		QuotaByteCount: 3,
		QuotaKeyCount:  3,
	}

	// expected quota output
	expected := &NamespaceQuota{
		Key:            testNamespaceQuota.Key,
		QuotaByteCount: testNamespaceQuota.QuotaByteCount,
		QuotaKeyCount:  testNamespaceQuota.QuotaKeyCount,
		UsageByteCount: 0,
		UsageKeyCount:  0,
	}

	// Add that to the tree
	ti := NewQuotaTreeIndex(zap.NewExample())
	_, isAdded := ti.Put(testNamespaceQuota)
	if isAdded {
		t.Errorf("error while setting quota, quota add status: %t", isAdded)
	}
	// received quota
	actual := ti.Get([]byte(testKey))

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("quota = %+v, want %+v", actual, expected)
	}

	// now since the quota is already in the tree, we are required to test overriding of quotas
	// Create a sample quota
	testNamespaceQuota.QuotaByteCount = 5
	testNamespaceQuota.QuotaKeyCount = 5

	// expected quota output
	expected.QuotaByteCount = 5
	expected.QuotaKeyCount = 5

	// Update the already existing quota in the tree
	_, isUpdated := ti.Put(testNamespaceQuota)
	if isUpdated {
		t.Errorf("error while setting quota, status: %t", isUpdated)
	}
	// received quota
	actual = ti.Get([]byte(testKey))

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("quota = %+v, want %+v", actual, expected)
	}
}

// TestKeyIndexDelete test key index delete tests if the delete
// operation was successfully and correctly performed
func TestKeyIndexDelete(t *testing.T) {
	testKey := "foo"

	// Create a sample quota
	testNamespaceQuota := &NamespaceQuota{
		Key:            []byte(testKey),
		QuotaByteCount: 3,
		QuotaKeyCount:  3,
	}

	// expected quota output
	expected := &NamespaceQuota{
		Key:            testNamespaceQuota.Key,
		QuotaByteCount: testNamespaceQuota.QuotaByteCount,
		QuotaKeyCount:  testNamespaceQuota.QuotaKeyCount,
		UsageByteCount: 0,
		UsageKeyCount:  0,
	}

	// Add that to the tree
	ti := NewQuotaTreeIndex(zap.NewExample())
	_, status := ti.Put(testNamespaceQuota)
	if status {
		t.Errorf("error while setting quota")
	}

	// received quota after delete
	actual := ti.Delete([]byte(testKey))

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("quota = %+v, want %+v", actual, expected)
	}

	// now deleting the quota should result a nil value
	nilValue := ti.Delete([]byte(testKey))

	if nilValue != nil {
		t.Errorf("quota = %+v, want %+v", nilValue, nil)
	}
}

// TestCheckQuotaPath test key index and check quota check path
func TestCheckQuotaPath(t *testing.T) {
	testKey := "foo"
	currKey := ""
	expectedKey := "foo%foo%foo%foo%foo"
	ti := NewQuotaTreeIndex(zap.NewExample())
	// Adding 5 keys
	for i := 5; i > 0; i-- {
		currKey += testKey
		item := &NamespaceQuota{
			Key:            []byte(currKey),
			QuotaByteCount: uint64(i),
			QuotaKeyCount:  uint64(i),
		}
		_, status := ti.Put(item)
		if status {
			t.Errorf("error while setting quota")
		}
		currKey += "%"
	}

	// Negative case where the last quota was violated and key cannot be added
	actualQuota := ti.CheckQuotaPath([]byte(currKey), 2, 2)
	if string(actualQuota.Key) != expectedKey {
		t.Errorf("expected %s got %s", expectedKey, string(actualQuota.Key))
	}

	// Pass case, where key can be added, as there are no violations
	actualQuota = ti.CheckQuotaPath([]byte(currKey), 1, 1)
	if actualQuota != nil {
		t.Errorf("expected: quota not exceeded, got: quota exceeded for key %s", string(actualQuota.Key))
	}
}

// TestUpdateQuotaPath test key index and update quota path
func TestUpdateQuotaPath(t *testing.T) {
	testKey := "foo"
	currKey := ""
	ti := NewQuotaTreeIndex(zap.NewExample())
	// Adding 5 keys
	for i := 5; i > 0; i-- {
		currKey += testKey
		item := &NamespaceQuota{
			Key:            []byte(currKey),
			QuotaByteCount: uint64(i),
			QuotaKeyCount:  uint64(i),
		}
		_, status := ti.Put(item)
		if status {
			t.Errorf("error while setting quota")
		}
		currKey += "%"
	}

	// Negative case where the last quota was violated and key cannot be added
	ti.UpdateQuotaPath([]byte(currKey), 1, 1)

	currKey = ""
	for i := 5; i > 0; i-- {
		currKey += testKey
		item := ti.Get([]byte(currKey))
		if item.UsageByteCount != 1 && item.UsageKeyCount != 1 {
			t.Errorf("error while setting quota")
		}
		currKey += "%"
	}
}
