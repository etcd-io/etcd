// Copyright 2015 CoreOS, Inc.
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

package clientv3

type KV interface {
	// PUT puts a key-value pair into etcd.
	// Note that key,value can be plain bytes array and string is
	// an immutable representation of that bytes array.
	// To get a string of bytes, do string([]byte(0x10, 0x20)).
	Put(key, val string) (PutResponse, error)

	// Range gets the keys [key, end) in the range at rev.
	// If revev <=0, range gets the keys at currentRev.
	// Limit limits the number of keys returned.
	// If the required rev is compacted, ErrCompacted will be returned.
	Range(key, end string, limit, rev int64, sort SortOption) (RangeResponse, error)

	// Get is like Range. A shortcut for ranging single key like [key, key+1).
	Get(key, limit, rev int64) (RangeResponse, error)

	// DeleteRange deletes the given range [key, end).
	DeleteRange(key, end string) (DeleteRangeResponse, error)

	// Delete is like DeleteRange. A shortcut for deleting single key like [key, key+1).
	Delete(key string) (DeleteRangeResponse, error)

	// Compact compacts etcd KV history before the given rev.
	Compact(rev int64) error

	// Txn creates a transaction.
	Txn() Txn
}

//
// Tx.If(
//  CmpValue(k1, ">", v1),
//  CmpVersion(k1, "=", 2)
// ).Then(
//  OpPut(k2,v2), OpPut(k3,v3)
// ).Else(
//  OpPut(k4,v4), OpPut(k5,v5)
// ).Commit()
type Txn interface {
	// If takes a list of comparison. If all comparisons passed in succeed,
	// the operations passed into Then() will be executed. Or the operations
	// passed into Else() will be executed.
	If(cs ...Compare) Txn

	// Then takes a list of operations. The Ops list will be executed, if the
	// comparisons passed in If() succeed.
	Then(ops ...Op) Txn

	// Else takes a list of operations. The Ops list will be executed, if the
	// comparisons passed in If() fail.
	Else(ops ...Op) Txn

	// Commit tries to commit the transaction.
	Commit() (TxnResponse, error)

	// TODO: add a Do for shortcut the txn without any condition?
}

type Compare struct {
}

type Op struct {
}

type PutResponse struct {
}

type DeleteRangeResponse struct {
}

type RangeResponse struct {
}

type TxnResponse struct {
}

type SortTarget int
type SortOrder int

type SortOption struct {
	Target SortTarget
	Order  SortOrder
}
