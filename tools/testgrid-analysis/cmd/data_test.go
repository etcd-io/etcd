// Copyright 2024 The etcd Authors
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

import (
	"testing"
)

func TestShortenTestName(t *testing.T) {
	tests := []struct {
		testName  string
		shortName string
	}{
		{
			testName:  "go.etcd.io/etcd/tests/v3/common.TestKVGet/ClientTLS",
			shortName: "common.TestKVGet/ClientTLS",
		},
		{
			testName:  "go.etcd.io/etcd/tests/v3/common.TestKVDelete/ClientTLS",
			shortName: "common.TestKVDelete/ClientTLS",
		},
		{
			testName:  "go.etcd.io/etcd/tests/v3/common.TestLeaseGrantAndList/ClientAutoTLS/many_leases",
			shortName: "common.TestLeaseGrantAndList/ClientAutoTLS/many_leases",
		},
		{
			testName:  "go.etcd.io/etcd/tests/v3/common.TestMoveLeaderWithInvalidAuth",
			shortName: "common.TestMoveLeaderWithInvalidAuth",
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			shortName := shortenTestName(tt.testName)
			if shortName != tt.shortName {
				t.Errorf("Want %s, got %s", tt.shortName, shortName)
			}
		})
	}
}
