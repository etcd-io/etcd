// Copyright 2023 The etcd Authors
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

package concurrency

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name string
		stm  *stmSerializable
		in   []string
		resp string
	}{
		{
			name: "Empty keys returns empty string",
			stm:  &stmSerializable{},
			in:   []string{},
			resp: "",
		},
		{
			name: "Nil keys returns empty string",
			stm:  &stmSerializable{},
			in:   nil,
			resp: "",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			resp := test.stm.Get(test.in...)

			assert.Equal(t, test.resp, resp)
		})
	}
}
