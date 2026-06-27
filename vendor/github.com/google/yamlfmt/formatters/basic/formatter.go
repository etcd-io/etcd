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

package basic

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/google/yamlfmt"
	yamlFeature "github.com/google/yamlfmt/formatters/basic/features"
	"github.com/google/yamlfmt/pkg/yaml"
	"github.com/mitchellh/mapstructure"
)

const BasicFormatterType string = "basic"

type BasicFormatter struct {
	Config       *Config
	Features     yamlfmt.FeatureList
	YAMLFeatures yamlFeature.YAMLFeatureList
}

// yamlfmt.Formatter interface

func (f *BasicFormatter) Type() string {
	return BasicFormatterType
}

func (f *BasicFormatter) Format(input []byte) ([]byte, error) {
	// Run all features with BeforeActions
	ctx := context.Background()
	ctx, yamlContent, err := f.Features.ApplyFeatures(ctx, input, yamlfmt.FeatureApplyBefore)
	if err != nil {
		return nil, err
	}

	// Format the yaml content
	reader := bytes.NewReader(yamlContent)
	decoder := f.getNewDecoder(reader)
	documents := []yaml.Node{}
	for {
		var docNode yaml.Node
		err := decoder.Decode(&docNode)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		documents = append(documents, docNode)
	}

	if len(documents) == 0 {
		return input, nil
	}

	// Run all YAML features.
	for _, d := range documents {
		if err := f.YAMLFeatures.ApplyFeatures(d); err != nil {
			return nil, err
		}
	}

	var b bytes.Buffer
	e := f.getNewEncoder(&b)
	for _, doc := range documents {
		err := e.Encode(&doc)
		if err != nil {
			return nil, err
		}
	}

	// Run all features with AfterActions
	_, resultYaml, err := f.Features.ApplyFeatures(ctx, b.Bytes(), yamlfmt.FeatureApplyAfter)
	if err != nil {
		return nil, err
	}

	return resultYaml, nil
}

func (f *BasicFormatter) getNewDecoder(reader io.Reader) *yaml.Decoder {
	d := yaml.NewDecoder(reader)
	if f.Config.ScanFoldedAsLiteral {
		d.SetScanBlockScalarAsLiteral(true)
	}
	return d
}

func (f *BasicFormatter) getNewEncoder(buf *bytes.Buffer) *yaml.Encoder {
	e := yaml.NewEncoder(buf)
	e.SetIndent(f.Config.Indent)

	if f.Config.LineLength > 0 {
		e.SetWidth(f.Config.LineLength)
	}

	if f.Config.LineEnding == yamlfmt.LineBreakStyleCRLF {
		e.SetLineBreakStyle(yaml.LineBreakStyleCRLF)
	}

	e.SetExplicitDocumentStart(f.Config.IncludeDocumentStart)
	e.SetAssumeBlockAsLiteral(f.Config.ScanFoldedAsLiteral)
	e.SetIndentlessBlockSequence(f.Config.IndentlessArrays)
	e.SetDropMergeTag(f.Config.DropMergeTag)
	e.SetPadLineComments(f.Config.PadLineComments)

	if f.Config.ArrayIndent > 0 {
		e.SetArrayIndent(f.Config.ArrayIndent)
	}
	e.SetIndentRootArray(f.Config.IndentRootArray)

	// Yes I know I could SetCorrectAliasKeys(!f.Config.DisableAliasKeyCorrection)
	// but I know myself and I know I'll get confused and have to go look up
	// the source again next time I look and forget.
	if !f.Config.DisableAliasKeyCorrection {
		e.SetCorrectAliasKeys(true)
	}

	return e
}

func (f *BasicFormatter) ConfigMap() (map[string]any, error) {
	configMap := map[string]any{}
	err := mapstructure.Decode(f.Config, &configMap)
	if err != nil {
		return nil, err
	}
	configMap["type"] = BasicFormatterType
	return configMap, err
}
