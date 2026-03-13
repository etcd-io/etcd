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

package pbutil

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestMarshaler(t *testing.T) {
	data := []byte("test data")
	m := &fakeMarshaler{data: data}
	g := MustMarshal(m)
	assert.Truef(t, reflect.DeepEqual(g, data), "data = %s, want %s", g, m.data)
}

func TestMarshalerPanic(t *testing.T) {
	defer func() {
		assert.NotNilf(t, recover(), "recover = nil, want error")
	}()
	m := &fakeMarshaler{err: errors.New("blah")}
	MustMarshal(m)
}

func TestUnmarshaler(t *testing.T) {
	data := []byte("test data")
	m := &fakeUnmarshaler{}
	MustUnmarshal(m, data)
	assert.Truef(t, reflect.DeepEqual(m.data, data), "data = %s, want %s", m.data, data)
}

func TestUnmarshalerPanic(t *testing.T) {
	defer func() {
		assert.NotNilf(t, recover(), "recover = nil, want error")
	}()
	m := &fakeUnmarshaler{err: errors.New("blah")}
	MustUnmarshal(m, nil)
}

func TestProtoMarshaler(t *testing.T) {
	m := wrapperspb.Int64(7)
	g := MustMarshalMessage(m)
	w, err := proto.Marshal(m)
	require.NoError(t, err)
	assert.Equal(t, w, g)
}

func TestProtoUnmarshaler(t *testing.T) {
	data := MustMarshalMessage(wrapperspb.Int64(9))
	m := &wrapperspb.Int64Value{}
	MustUnmarshalMessage(m, data)
	assert.Equal(t, int64(9), m.Value)
}

func TestProtoUnmarshalerPanic(t *testing.T) {
	defer func() {
		assert.NotNilf(t, recover(), "recover = nil, want error")
	}()
	m := &wrapperspb.Int64Value{}
	MustUnmarshalMessage(m, []byte("not-a-protobuf"))
}

func TestMaybeProtoUnmarshal(t *testing.T) {
	m := &wrapperspb.Int64Value{}
	ok := MaybeUnmarshalMessage(m, MustMarshalMessage(wrapperspb.Int64(11)))
	assert.True(t, ok)
	assert.Equal(t, int64(11), m.Value)

	ok = MaybeUnmarshalMessage(m, []byte("bad"))
	assert.False(t, ok)
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		b    *bool
		wb   bool
		wset bool
	}{
		{nil, false, false},
		{Boolp(true), true, true},
		{Boolp(false), false, true},
	}
	for i, tt := range tests {
		b, set := GetBool(tt.b)
		assert.Equalf(t, b, tt.wb, "#%d: value = %v, want %v", i, b, tt.wb)
		assert.Equalf(t, set, tt.wset, "#%d: set = %v, want %v", i, set, tt.wset)
	}
}

type fakeMarshaler struct {
	data []byte
	err  error
}

func (m *fakeMarshaler) Marshal() ([]byte, error) {
	return m.data, m.err
}

type fakeUnmarshaler struct {
	data []byte
	err  error
}

func (m *fakeUnmarshaler) Unmarshal(data []byte) error {
	m.data = data
	return m.err
}
