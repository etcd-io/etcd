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

package integration_test

import (
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/integration"
)

func TestBeforeTestWithoutLeakDetection(t *testing.T) {
	integration.BeforeTest(t, integration.WithoutGoLeakDetection(), integration.WithoutSkipInShort())
	// Intentional leak that should get ignored
	go time.Sleep(2 * time.Second)
}
