// Copyright 2018 The etcd Authors
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

package backend

import (
	bolt "go.etcd.io/bbolt"
)

type concurrentReadTx struct {
	tx *bolt.Tx
}

func (rt *concurrentReadTx) Lock()   {}
func (rt *concurrentReadTx) Unlock() { rt.tx.Rollback() }

func (rt *concurrentReadTx) UnsafeRange(bucketName, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	bucket := rt.tx.Bucket(bucketName)
	if bucket == nil {
		plog.Fatalf("bucket %s does not exist", bucketName)
	}
	return unsafeRange(bucket.Cursor(), key, endKey, limit)
}

func (rt *concurrentReadTx) UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error {
	return unsafeForEach(rt.tx, bucketName, visitor)
}

func (m *MonitoredReadTx) UnsafeRange(bucketName, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	return m.Tx.UnsafeRange(bucketName, key, endKey, limit)
}
func (m *MonitoredReadTx) UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error {
	return m.Tx.UnsafeForEach(bucketName, visitor)
}
