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

package config

type V2DeprecationEnum string

const (
	// Default in v3.5.  Issues a warning if v2store have meaningful content.
	V2_DEPR_0_NOT_YET = V2DeprecationEnum("not-yet")
	// Default in v3.6.  Meaningful v2 state is not allowed.
	// The V2 files are maintained for v3.5 rollback.
	V2_DEPR_1_WRITE_ONLY = V2DeprecationEnum("write-only")
	// V2_DEPR_1_WRITE_ONLY_SKIP_CHECK is like V2_DEPR_1_WRITE_ONLY, but bypasses the v2 content check.
	// Use only for 3.5 -> 3.6 upgrades with existing v2 data.
	// WARNING: Users should read the 3.5 -> 3.6 upgrade guide and use this option at their own risk.
	V2_DEPR_1_WRITE_ONLY_SKIP_CHECK = V2DeprecationEnum("write-only-skip-check")
	// V2store is WIPED if found !!!
	V2_DEPR_1_WRITE_ONLY_DROP = V2DeprecationEnum("write-only-drop-data")
	// V2store is neither written nor read. Usage of this configuration is blocking
	// ability to rollback to etcd v3.5.
	V2_DEPR_2_GONE = V2DeprecationEnum("gone")

	V2_DEPR_DEFAULT = V2_DEPR_0_NOT_YET
)

func (e V2DeprecationEnum) IsAtLeast(v2d V2DeprecationEnum) bool {
	return e.level() >= v2d.level()
}

func (e V2DeprecationEnum) level() int {
	switch e {
	case V2_DEPR_0_NOT_YET:
		return 0
	case V2_DEPR_1_WRITE_ONLY, V2_DEPR_1_WRITE_ONLY_SKIP_CHECK:
		return 1
	case V2_DEPR_1_WRITE_ONLY_DROP:
		return 2
	case V2_DEPR_2_GONE:
		return 3
	}
	panic("Unknown V2DeprecationEnum: " + e)
}
