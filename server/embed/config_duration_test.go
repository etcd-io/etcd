// Copyright 2025 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package embed

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalJSON(t *testing.T) {
	var d1 ConfigDuration
	data := []byte(`"10s"`)
	err := d1.UnmarshalJSON(data)
	require.EqualValues(t, 10*time.Second, d1)
	require.NoError(t, err)

	var d2 ConfigDuration
	data = []byte("10000000000")
	err = d2.UnmarshalJSON(data)
	require.EqualValues(t, 10*time.Second, d2)
	require.NoError(t, err)

	var d3 ConfigDuration
	data = []byte("")
	err = d3.UnmarshalJSON(data)
	require.Error(t, err)
	require.EqualValues(t, 0, d3)

	var d4 ConfigDuration
	data = []byte(`"non-duration string"`)
	err = d4.UnmarshalJSON(data)
	require.Error(t, err)
	require.EqualValues(t, 0, d4)

	var d5 ConfigDuration
	data = []byte(`["10s"]`)
	err = d5.UnmarshalJSON(data)
	require.NoError(t, err)
	require.EqualValues(t, 0, d5)
}
