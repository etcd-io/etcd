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

import (
	"errors"
	"fmt"

	"github.com/google/yamlfmt/pkg/yaml"
)

type QuoteStyle string

const (
	SingleQuoteStyle QuoteStyle = "single"
	DoubleQuoteStyle QuoteStyle = "double"
)

var ErrUnrecognizedQuoteStyle = errors.New("unrecognized quote style")

func FeatureForceQuoteStyle(style QuoteStyle) (YAMLFeatureFunc, error) {
	var fromStyle, toStyle yaml.Style
	switch style {
	case SingleQuoteStyle:
		fromStyle = yaml.DoubleQuotedStyle
		toStyle = yaml.SingleQuotedStyle
	case DoubleQuoteStyle:
		fromStyle = yaml.SingleQuotedStyle
		toStyle = yaml.DoubleQuotedStyle
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnrecognizedQuoteStyle, style)
	}
	var forceStyle YAMLFeatureFunc
	forceStyle = func(n yaml.Node) error {
		var err error
		for _, c := range n.Content {
			if c.Style == fromStyle {
				c.Style = toStyle
			}
			err = forceStyle(*c)
		}
		return err
	}
	return forceStyle, nil
}
