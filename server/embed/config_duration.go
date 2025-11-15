// Copyright 2025 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package embed

import (
	"encoding/json"
	"time"
)

type ConfigDuration time.Duration

func (d *ConfigDuration) UnmarshalJSON(b []byte) error {
	var v any
	var err error
	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = ConfigDuration(value)
		return nil
	case string:
		td, err := time.ParseDuration(value)
		if err == nil {
			*d = ConfigDuration(td)
		}
		return err
	}
	return nil
}
