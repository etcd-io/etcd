// Copyright 2025 Google LLC
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

type SequenceStyle string

const (
	SequenceStyleBlock SequenceStyle = "block"
	SequenceStyleFlow  SequenceStyle = "flow"
)

var ErrUnrecognizedSequenceStyle = errors.New("unrecognized sequence style")

func FeatureForceSequenceStyle(style SequenceStyle) (YAMLFeatureFunc, error) {
	var styleVal yaml.Style
	switch style {
	case SequenceStyleFlow:
		styleVal = yaml.FlowStyle
	case SequenceStyleBlock:
		// In the AST, a Sequence node marked with no style
		// is rendered in block style. So this functionally
		// is force setting to 0 which causes it to be rendered
		// as block style.
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnrecognizedSequenceStyle, style)
	}
	var forceStyle YAMLFeatureFunc
	forceStyle = func(n yaml.Node) error {
		var err error
		for _, c := range n.Content {
			if c.Kind == yaml.SequenceNode {
				c.Style = styleVal
			}
			err = forceStyle(*c)
		}
		return err
	}
	return forceStyle, nil
}
