// Copyright 2015 The etcd Authors
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

// Package pbutil defines interfaces for handling Protocol Buffer objects.
package pbutil

import (
	"bytes"

	"google.golang.org/grpc/metadata"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/coreos/pkg/capnslog"
	"github.com/opentracing/opentracing-go"
)

var (
	plog = capnslog.NewPackageLogger("github.com/coreos/etcd/pkg", "flags")
)

type Marshaler interface {
	Marshal() (data []byte, err error)
}

type Unmarshaler interface {
	Unmarshal(data []byte) error
}

func MustMarshal(m Marshaler) []byte {
	d, err := m.Marshal()
	if err != nil {
		plog.Panicf("marshal should never fail (%v)", err)
	}
	return d
}

func MustUnmarshal(um Unmarshaler, data []byte) {
	if err := um.Unmarshal(data); err != nil {
		plog.Panicf("unmarshal should never fail (%v)", err)
	}
}

func MaybeUnmarshal(um Unmarshaler, data []byte) bool {
	if err := um.Unmarshal(data); err != nil {
		return false
	}
	return true
}

func GetBool(v *bool) (vv bool, set bool) {
	if v == nil {
		return false, false
	}
	return *v, true
}

func Boolp(b bool) *bool { return &b }

func InjectSpanIntoMessage(sp opentracing.Span, msg *raftpb.Message) error {
	buf := bytes.NewBuffer(nil)
	err := sp.Tracer().Inject(sp, opentracing.Binary, buf)
	if err != nil {
		return err
	}
	msg.TraceContext = buf.Bytes()
	return err
}

func StartSpanFromMessage(opName string, msg raftpb.Message) opentracing.Span {
	buf := bytes.NewReader(msg.TraceContext)
	sp, err := opentracing.GlobalTracer().Join(opName, opentracing.Binary, buf)
	if err != nil {
		return opentracing.StartSpan(opName)
	}
	return sp
}

// MetadataReaderWriter implements opentracing.TextMapReader
// and opentracing.TextMapWriter. Used for gRPC metadata.
type MetadataReaderWriter struct {
	*metadata.MD
}

func (w MetadataReaderWriter) Set(key, val string) {
	(*w.MD)[key] = append((*w.MD)[key], val)
}

func (w MetadataReaderWriter) ForeachKey(handler func(key, val string) error) error {
	for k, vals := range *w.MD {
		for _, v := range vals {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}
