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

package embed

import (
	"testing"
)

const neverSampleDescription = "AlwaysOffSampler"

func TestDetermineSampler(t *testing.T) {
	tests := []struct {
		name                   string
		sampleRate             int
		wantSamplerDescription string
	}{
		{
			name:                   "sample rate is disabled",
			sampleRate:             0,
			wantSamplerDescription: neverSampleDescription,
		},
		{
			name:                   "sample rate is 100",
			sampleRate:             100,
			wantSamplerDescription: "TraceIDRatioBased{0.0001}",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sampler := determineSampler(tc.sampleRate)
			if tc.wantSamplerDescription != sampler.Description() {
				t.Errorf("tracing sampler was not as expected; expected sampler: %#+v, got sampler: %#+v", tc.wantSamplerDescription, sampler.Description())
			}
		})
	}
}

func TestTracingConfig(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate int
		wantErr    bool
	}{
		{
			name:       "invalid - sample rate is less than 0",
			sampleRate: -1,
			wantErr:    true,
		},
		{
			name:       "invalid - sample rate is more than allowed value",
			sampleRate: maxSamplingRatePerMillion + 1,
			wantErr:    true,
		},
		{
			name:       "valid - sample rate is 100",
			sampleRate: 100,
			wantErr:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTracingConfig(tc.sampleRate)
			if err == nil && tc.wantErr {
				t.Errorf("expected error got (%v) error", err)
			}
			if err != nil && !tc.wantErr {
				t.Errorf("expected no errors, got error: (%v)", err)
			}
		})
	}
}
