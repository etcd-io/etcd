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

// Package pb contains clone helpers for the current gogo-generated protobuf
// messages used by client/v3.
//
// TODO: remove this package after client/v3 switches to the official
// protobuf generator.
package pb

import (
	gogoproto "github.com/gogo/protobuf/proto"

	v3pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

// TODO: use the official protobuf clone path after client/v3 stops using
// gogo-generated messages.
func CloneCompare(c *v3pb.Compare) *v3pb.Compare {
	if c == nil {
		return nil
	}
	return gogoproto.Clone(c).(*v3pb.Compare)
}
