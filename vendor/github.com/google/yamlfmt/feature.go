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

package yamlfmt

import (
	"context"
	"fmt"
)

type FeatureFunc func(context.Context, []byte) (context.Context, []byte, error)

type Feature struct {
	Name         string
	BeforeAction FeatureFunc
	AfterAction  FeatureFunc
}

type FeatureList []Feature

type FeatureApplyMode string

var (
	FeatureApplyBefore FeatureApplyMode = "Before"
	FeatureApplyAfter  FeatureApplyMode = "After"
)

type FeatureApplyError struct {
	err         error
	featureName string
	mode        FeatureApplyMode
}

func (e *FeatureApplyError) Error() string {
	return fmt.Sprintf("Feature %s %sAction failed with error: %v", e.featureName, e.mode, e.err)
}

func (e *FeatureApplyError) Unwrap() error {
	return e.err
}

func (fl FeatureList) ApplyFeatures(ctx context.Context, input []byte, mode FeatureApplyMode) (context.Context, []byte, error) {
	// Declare err here so the result variable doesn't get shadowed in the loop
	var err error
	result := make([]byte, len(input))
	copy(result, input)
	for _, feature := range fl {
		if mode == FeatureApplyBefore {
			if feature.BeforeAction != nil {
				ctx, result, err = feature.BeforeAction(ctx, result)
			}
		} else {
			if feature.AfterAction != nil {
				ctx, result, err = feature.AfterAction(ctx, result)
			}
		}

		if err != nil {
			return ctx, nil, &FeatureApplyError{
				err:         err,
				featureName: feature.Name,
				mode:        mode,
			}
		}
	}
	return ctx, result, nil
}
