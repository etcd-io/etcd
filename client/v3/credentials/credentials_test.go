// Copyright 2022 The etcd Authors
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

package credentials

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
)

func TestUpdateAuthToken(t *testing.T) {
	bundle := NewPerRPCCredentialBundle()
	ctx := t.Context()

	metadataBeforeUpdate, _ := bundle.PerRPCCredentials().GetRequestMetadata(ctx)
	assert.Empty(t, metadataBeforeUpdate)

	bundle.UpdateAuthToken("abcdefg")

	metadataAfterUpdate, _ := bundle.PerRPCCredentials().GetRequestMetadata(ctx)
	assert.Equal(t, "abcdefg", metadataAfterUpdate[rpctypes.TokenFieldNameGRPC])
}
