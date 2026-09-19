// Copyright 2026 The etcd Authors
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

package cmd

import "testing"

func TestValidateWatchFlags(t *testing.T) {
	tests := []struct {
		name             string
		streams          int
		watchesPerStream int
		watchedKeyTotal  int
		keySpaceSize     int
		expectErr        bool
	}{
		{"all positive", 1, 1, 1, 1, false},
		{"zero streams", 0, 1, 1, 1, true},
		{"negative streams", -1, 1, 1, 1, true},
		{"zero watch-per-stream", 1, 0, 1, 1, true},
		{"negative watch-per-stream", 1, -1, 1, 1, true},
		{"zero watched-key-total", 1, 1, 0, 1, true},
		{"negative watched-key-total", 1, 1, -1, 1, true},
		{"zero key-space-size", 1, 1, 1, 0, true},
		{"negative key-space-size", 1, 1, 1, -1, true},
	}

	// Save and restore the package-level flag variables since they are
	// shared global state read by validateWatchFlags.
	origStreams, origWatchesPerStream := watchStreams, watchWatchesPerStream
	origWatchedKeyTotal, origKeySpaceSize := watchedKeyTotal, watchKeySpaceSize
	defer func() {
		watchStreams, watchWatchesPerStream = origStreams, origWatchesPerStream
		watchedKeyTotal, watchKeySpaceSize = origWatchedKeyTotal, origKeySpaceSize
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watchStreams = tt.streams
			watchWatchesPerStream = tt.watchesPerStream
			watchedKeyTotal = tt.watchedKeyTotal
			watchKeySpaceSize = tt.keySpaceSize

			err := validateWatchFlags()
			if tt.expectErr && err == nil {
				t.Errorf("validateWatchFlags() = nil, want error")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("validateWatchFlags() = %v, want nil", err)
			}
		})
	}
}
