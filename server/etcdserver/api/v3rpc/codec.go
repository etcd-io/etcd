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

package v3rpc

import "github.com/golang/protobuf/proto" //nolint:staticcheck // TODO: remove for a supported version

// vtMarshaler and vtUnmarshaler are implemented by messages generated with
// protoc-gen-go-vtproto. When a message provides these zero-reflection methods
// we use them; otherwise we fall back to the standard reflection-based codec.
type vtMarshaler interface{ MarshalVT() ([]byte, error) }

type vtUnmarshaler interface{ UnmarshalVT([]byte) error }

type codec struct{}

func (c *codec) Marshal(v any) ([]byte, error) {
	var (
		b   []byte
		err error
	)
	if m, ok := v.(vtMarshaler); ok {
		b, err = m.MarshalVT()
	} else {
		b, err = proto.Marshal(v.(proto.Message))
	}
	sentBytes.Add(float64(len(b)))
	return b, err
}

func (c *codec) Unmarshal(data []byte, v any) error {
	receivedBytes.Add(float64(len(data)))
	if m, ok := v.(vtUnmarshaler); ok {
		return m.UnmarshalVT(data)
	}
	return proto.Unmarshal(data, v.(proto.Message))
}

func (c *codec) String() string {
	return "proto"
}
