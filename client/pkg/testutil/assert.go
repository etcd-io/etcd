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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func copyToInterface(msg ...string) []interface{} {
	newMsg := make([]interface{}, len(msg))
	for i, v := range msg {
		newMsg[i] = v
	}
	return newMsg
}

func AssertNil(t *testing.T, v interface{}) {
	t.Helper()
	assert.Nil(t, v)
}

func AssertNotNil(t *testing.T, v interface{}) {
	t.Helper()
	if v == nil {
		t.Fatalf("expected non-nil, got %+v", v)
	}
}

func AssertTrue(t *testing.T, v bool, msg ...string) {
	t.Helper()
	newMsg := copyToInterface(msg...)
	assert.Equal(t, true, v, newMsg)
}

func AssertFalse(t *testing.T, v bool, msg ...string) {
	t.Helper()
	newMsg := copyToInterface(msg...)
	assert.Equal(t, false, v, newMsg)
}

func isNil(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() != reflect.Struct && rv.IsNil()
}
