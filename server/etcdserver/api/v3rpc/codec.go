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

import (
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/encoding"
)

// RegisterCodec encapsulates the `encoding.RegisterCodec`.
//
// `grpc.CustomCodec` has already been deprecated in gRPC 1.47. Users are
//  supposed to use `encoding.RegisterCodec` to register customized codec.
func RegisterCodec(name string) {
	encoding.RegisterCodec(&codec{name})
}

// codec needs to implement interface `Codec` when registering codec using
// `encoding.RegisterCodec`, please refer to
// https://github.com/grpc/grpc-go/blob/v1.47.0/encoding/encoding.go#L86.
type codec struct {
	name string
}

func (c *codec) Marshal(v interface{}) ([]byte, error) {
	b, err := proto.Marshal(v.(proto.Message))
	sentBytes.Add(float64(len(b)))
	return b, err
}

func (c *codec) Unmarshal(data []byte, v interface{}) error {
	receivedBytes.Add(float64(len(data)))
	return proto.Unmarshal(data, v.(proto.Message))
}

func (c *codec) Name() string {
	return c.name
}

func (c *codec) String() string {
	return c.name
}
