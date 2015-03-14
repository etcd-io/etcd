// Copyright 2015 CoreOS, Inc.
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

package logutil

import (
	"log"
	"testing"
)

func TestInitLogger(t *testing.T) {
	initialFlags := log.Flags()
	defer log.SetFlags(initialFlags)

	tests := []struct {
		config   *Config
		expected int
	}{
		{&Config{DisableTimestamps: false}, log.LstdFlags},
		{&Config{DisableTimestamps: true}, 0},
		{nil, log.LstdFlags},
	}

	for _, tt := range tests {
		InitLogger(tt.config)
		flags := log.Flags()

		if flags != tt.expected {
			t.Errorf("expected: %v, got %v", tt.expected, flags)
		}
	}
}

func TestGetFlags(t *testing.T) {
	tests := []struct {
		config   *Config
		expected int
	}{
		{&Config{DisableTimestamps: false}, log.LstdFlags},
		{&Config{DisableTimestamps: true}, 0},
		{nil, log.LstdFlags},
	}

	for _, tt := range tests {
		got := getFlags(tt.config)
		if got != tt.expected {
			t.Errorf("expected: %v, got %v", tt.expected, got)
		}
	}
}
