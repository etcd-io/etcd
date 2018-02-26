// Copyright 2017 The etcd Authors
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
	bolt "github.com/coreos/bbolt"
)

type staleConcurrentReadTx struct {
	tx *bolt.Tx
}

func (rt *staleConcurrentReadTx) Lock() {}
func (rt *staleConcurrentReadTx) Unlock() {
	err := rt.tx.Rollback()
	if err != nil {
		panic(err)
	}
}

func (rt *staleConcurrentReadTx) UnsafeRange(bucketName, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	bucket := rt.tx.Bucket(bucketName)
	if bucket == nil {
		plog.Fatalf("bucket %s does not exist", bucketName)
	}
	return unsafeRange(bucket.Cursor(), key, endKey, limit)
}

func (rt *staleConcurrentReadTx) UnsafeForEach(bucketName []byte, visitor func(k, v []byte) error) error {
	return unsafeForEach(rt.tx, bucketName, visitor)

}
