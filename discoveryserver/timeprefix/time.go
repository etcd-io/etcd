// Copyright 2019 The etcd Authors
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

package timeprefix

import "time"

func Prefixes(before time.Duration, durationrange time.Duration) []string {
	ft := time.Now().UTC().Add(-1 * (before + durationrange))
	lt := time.Now().UTC().Add(-1 * before)

	return []string{ft.Format(time.RFC3339), lt.Format(time.RFC3339)}
}

func Now() string {
	return Past(0)
}

func Past(d time.Duration) string {
	return time.Now().Add(-1 * d).UTC().Format(time.RFC3339)
}
