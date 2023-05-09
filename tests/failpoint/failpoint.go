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

package failpoint

import (
	gofail "go.etcd.io/gofail/runtime"
)

var EmptyFailPoint = &failpointFunc{}

type Failpoint interface {
	Enable() error
	Disable() error
}

type failpointFunc struct {
	name    string
	payload string
}

func (f *failpointFunc) Enable() error {
	if f == nil || len(f.name) == 0 {
		return nil
	}
	return gofail.Enable(f.name, f.payload)
}

func (f *failpointFunc) Disable() error {
	if f == nil || len(f.name) == 0 {
		return nil
	}
	return gofail.Disable(f.name)
}

func NewFailpoint(name, payload string) *failpointFunc {
	return &failpointFunc{name, payload}
}
