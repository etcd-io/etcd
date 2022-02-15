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
	"go.etcd.io/etcd/server/v3/namespacequota/namespacequotapb"
	"go.etcd.io/etcd/server/v3/storage/backend"
)

func UnsafeCreateNamespaceQuotaBucket(tx backend.BatchTx) {
	tx.UnsafeCreateBucket(NamespaceQuota)
}

func MustUnsafeGetAllNamespaceQuotas(tx backend.ReadTx) []*namespacequotapb.NamespaceQuota {
	_, vs := tx.UnsafeRange(NamespaceQuota, []byte{0}, []byte{0xff}, -1)
	nqs := make([]*namespacequotapb.NamespaceQuota, 0, len(vs))
	for i := range vs {
		var nqpb namespacequotapb.NamespaceQuota
		err := nqpb.Unmarshal(vs[i])
		if err != nil {
			panic("failed to unmarshal namespace quota proto item")
		}
		nqs = append(nqs, &nqpb)
	}
	return nqs
}

func MustUnsafePutNamespaceQuota(tx backend.BatchTx, nqpb *namespacequotapb.NamespaceQuota) {
	key := nqpb.Key
	val, err := nqpb.Marshal()
	if err != nil {
		panic("failed to marshal namespace quota proto item")
	}
	tx.UnsafePut(NamespaceQuota, []byte(key), val)
}
