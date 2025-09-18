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

package engine

import (
	"encoding/json"

	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/engine/intf"
)

type report struct {
	Input   any   `json:"input,omitempty"`
	Results []any `json:"results,omitempty"`
}

// Diagnose runs all provided plugins and returns a JSON report.
// It logs plugin progress and individual results to stderr.
func Diagnose(input any, plugins []intf.Plugin) ([]byte, error) {
	rp := report{
		Input: input,
	}
	for _, plugin := range plugins {
		result := plugin.Diagnose()
		rp.Results = append(rp.Results, result)
	}

	return json.MarshalIndent(rp, "", "\t")
}
