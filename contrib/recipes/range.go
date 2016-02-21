// Copyright 2016 CoreOS, Inc.
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

package recipe

import (
	v3 "github.com/coreos/etcd/clientv3"
)

func withFirstCreate() []v3.OpOption { return withTop(v3.SortByCreatedRev, v3.SortAscend) }
func withLastCreate() []v3.OpOption  { return withTop(v3.SortByCreatedRev, v3.SortDescend) }
func withFirstKey() []v3.OpOption    { return withTop(v3.SortByKey, v3.SortAscend) }
func withLastKey() []v3.OpOption     { return withTop(v3.SortByKey, v3.SortDescend) }
func withFirstRev() []v3.OpOption    { return withTop(v3.SortByModifiedRev, v3.SortAscend) }
func withLastRev() []v3.OpOption     { return withTop(v3.SortByModifiedRev, v3.SortDescend) }

// withTop gets the first key over the get's prefix given a sort order
func withTop(target v3.SortTarget, order v3.SortOrder) []v3.OpOption {
	return []v3.OpOption{
		v3.WithPrefix(),
		v3.WithSort(target, order),
		v3.WithLimit(1)}
}
