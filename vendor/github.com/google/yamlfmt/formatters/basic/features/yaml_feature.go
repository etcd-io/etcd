// Copyright 2025 Google LLC
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

package features

import "github.com/google/yamlfmt/pkg/yaml"

// These features will directly use the `yaml.Node` type and
// as such are specific to this formatter.
type YAMLFeatureFunc func(yaml.Node) error
type YAMLFeatureList []YAMLFeatureFunc

func (y YAMLFeatureList) ApplyFeatures(node yaml.Node) error {
	for _, f := range y {
		// A little defensive programming, likely a condition that won't be hit.
		if f == nil {
			continue
		}
		if err := f(node); err != nil {
			return err
		}
	}
	return nil
}
