// Copyright 2017 The etcd Authors
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

package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// AssertNil
// Deprecated: use github.com/stretchr/testify/assert.Nil instead.
func AssertNil(t *testing.T, v any) {
	t.Helper()
	assert.Nil(t, v)
}

// AssertNotNil
// Deprecated: use github.com/stretchr/testify/require.NotNil instead.
func AssertNotNil(t *testing.T, v any) {
	t.Helper()
	if v == nil {
		t.Fatalf("expected non-nil, got %+v", v)
	}
}

// AssertTrue
// Deprecated: use github.com/stretchr/testify/assert.True instead.
func AssertTrue(t *testing.T, v bool, msg ...string) {
	t.Helper()
	assert.True(t, v, msg) //nolint:testifylint
}

// AssertFalse
// Deprecated: use github.com/stretchr/testify/assert.False instead.
func AssertFalse(t *testing.T, v bool, msg ...string) {
	t.Helper()
	assert.False(t, v, msg) //nolint:testifylint
}
