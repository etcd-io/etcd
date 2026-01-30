// Copyright 2025 The etcd Authors
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

package coverage_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func sortPatternTable(res map[Row]int) []Row {
	keys := slices.Collect(maps.Keys(res))
	slices.SortFunc(keys, func(a, b Row) int {
		if a.Pattern != b.Pattern {
			return strings.Compare(a.Pattern, b.Pattern)
		}
		if a.Method != b.Method {
			return strings.Compare(a.Method, b.Method)
		}
		return strings.Compare(a.Args, b.Args)
	})
	return keys
}

func TestSortPatternTable(t *testing.T) {
	want := []Row{
		{Pattern: "/registry/events/", Method: "", Args: ""},
		{Pattern: "/registry/events/{namespace}/", Method: "List", Args: ""},
		{Pattern: "/registry/events/{namespace}/{name}", Method: "Get", Args: ""},
		{Pattern: "/registry/health", Method: "Healthcheck", Args: ""},
		{Pattern: "/registry/masterleases/", Method: "List", Args: ""},
		{Pattern: "/registry/masterleases/{name}", Method: "Get", Args: ""},
		{Pattern: "/registry/{api-group}/{resource}/", Method: "List", Args: "XX"},
		{Pattern: "/registry/{api-group}/{resource}/", Method: "List", Args: "XXX"},
		{Pattern: "/registry/{api-group}/{resource}/{name}", Method: "Get", Args: ""},
		{Pattern: "/registry/{resource}", Method: "Get", Args: ""},
		{Pattern: "/registry/{resource}/", Method: "List", Args: ""},
		{Pattern: "/registry/{resource}/{namespace}/{name}", Method: "Get", Args: ""},
		{Pattern: "/registry/{resource}/{name}", Method: "Get", Args: ""},
		{Pattern: "compact_rev_key", Method: "Compaction", Args: ""},
	}
	res := make(map[Row]int, len(want))
	for _, r := range want {
		res[r] = 0
	}
	got := sortPatternTable(res)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Logf("\n+%#v", got)
		t.Errorf("Sort mismatch (-want +got):\n%s", diff)
	}
}
