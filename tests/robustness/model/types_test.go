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

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/types"
)

func TestMemberIDMatchesTypesID(t *testing.T) {
	raw := uint64(13770331908272521436)

	mID := MemberID(raw)
	jsonBytes, err := json.Marshal(mID)
	require.NoError(t, err)
	jsonStr := string(jsonBytes)

	tID := types.ID(raw)
	expectedHex := tID.String()
	require.JSONEq(t, "\""+expectedHex+"\"", jsonStr)
}

func TestMemberIDMatchesTypesIDUnmarshal(t *testing.T) {
	raw := uint64(13770331908272521999)
	mID := MemberID(raw)

	jsonBytes, err := json.Marshal(mID)
	require.NoError(t, err)

	tID := types.ID(raw)
	expectedHex := "\"" + tID.String() + "\""
	require.JSONEq(t, expectedHex, string(jsonBytes))

	var parsedID MemberID
	err = json.Unmarshal(jsonBytes, &parsedID)
	require.NoError(t, err)
	require.Equal(t, mID, parsedID)
}
