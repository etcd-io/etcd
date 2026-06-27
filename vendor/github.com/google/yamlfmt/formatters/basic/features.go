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
	"github.com/google/yamlfmt"
	yamlFeatures "github.com/google/yamlfmt/formatters/basic/features"
	"github.com/google/yamlfmt/internal/features"
	"github.com/google/yamlfmt/internal/hotfix"
)

func ConfigureFeaturesFromConfig(config *Config) yamlfmt.FeatureList {
	lineSep, err := config.LineEnding.Separator()
	if err != nil {
		lineSep = "\n"
	}
	configuredFeatures := []yamlfmt.Feature{}
	if config.RetainLineBreaks || config.RetainLineBreaksSingle {
		configuredFeatures = append(
			configuredFeatures,
			hotfix.MakeFeatureRetainLineBreak(lineSep, config.RetainLineBreaksSingle),
		)
	}
	if config.TrimTrailingWhitespace {
		configuredFeatures = append(
			configuredFeatures,
			features.MakeFeatureTrimTrailingWhitespace(lineSep),
		)
	}
	if config.EOFNewline {
		configuredFeatures = append(
			configuredFeatures,
			features.MakeFeatureEOFNewline(lineSep),
		)
	}
	if config.StripDirectives {
		configuredFeatures = append(
			configuredFeatures,
			hotfix.MakeFeatureStripDirectives(lineSep),
		)
	}
	return configuredFeatures
}

func ConfigureYAMLFeaturesFromConfig(config *Config) (yamlFeatures.YAMLFeatureList, error) {
	var featureList yamlFeatures.YAMLFeatureList

	if config.DisallowAnchors {
		featureList = append(featureList, yamlFeatures.Check)
	}

	if config.ForceArrayStyle != "" {
		sequenceStyleFeature, err := yamlFeatures.FeatureForceSequenceStyle(config.ForceArrayStyle)
		if err != nil {
			return featureList, err
		}
		featureList = append(featureList, sequenceStyleFeature)
	}

	if config.ForceQuoteStyle != "" {
		quoteStyleFeature, err := yamlFeatures.FeatureForceQuoteStyle(config.ForceQuoteStyle)
		if err != nil {
			return featureList, err
		}
		featureList = append(featureList, quoteStyleFeature)
	}

	return featureList, nil
}
