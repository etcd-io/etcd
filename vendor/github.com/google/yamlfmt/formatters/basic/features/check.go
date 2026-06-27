// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package features

import (
	"errors"
	"fmt"

	"github.com/google/yamlfmt/pkg/yaml"
)

func Check(n yaml.Node) error {
	if n.Kind == yaml.AliasNode {
		return errors.New("alias node found")
	}
	if n.Anchor != "" {
		return fmt.Errorf("node references anchor %q", n.Anchor)
	}
	for _, c := range n.Content {
		if err := Check(*c); err != nil {
			return err
		}
	}
	return nil
}
