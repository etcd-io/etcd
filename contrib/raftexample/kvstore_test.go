// Copyright 2016 The etcd Authors
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

package main

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_kvstore_snapshot(t *testing.T) {
	tm := map[string]string{"foo": "bar"}
	s := &kvstore{kvStore: tm}

	v, _ := s.Lookup("foo")
	require.Equalf(t, "bar", v, "foo has unexpected value, got %s", v)

	data, err := s.getSnapshot()
	require.NoError(t, err)
	s.kvStore = nil

	err = s.recoverFromSnapshot(data)
	require.NoError(t, err)
	v, _ = s.Lookup("foo")
	require.Equalf(t, "bar", v, "foo has unexpected value, got %s", v)
	require.Truef(t, reflect.DeepEqual(s.kvStore, tm), "store expected %+v, got %+v", tm, s.kvStore)
}
