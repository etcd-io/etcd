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

package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigureGetOutput(t *testing.T) {
	simple := &simplePrinter{}
	require.NoError(t, configureGetOutput(simple, false, false))
	require.False(t, simple.valueOnly)
	require.NoError(t, configureGetOutput(simple, false, true))
	require.True(t, simple.valueOnly)
	require.NoError(t, configureGetOutput(&fieldsPrinter{}, true, false))
}
