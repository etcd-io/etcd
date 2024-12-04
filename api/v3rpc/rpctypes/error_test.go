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

package rpctypes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestConvert(t *testing.T) {
	e1 := status.Error(codes.InvalidArgument, "etcdserver: key is not provided")
	e2 := ErrGRPCEmptyKey
	var e3 EtcdError
	errors.As(ErrEmptyKey, &e3)

	require.Equal(t, e1.Error(), e2.Error())
	if ev1, ok := status.FromError(e1); ok {
		require.Equal(t, ev1.Code(), e3.Code())
	}

	require.NotEqual(t, e1.Error(), e3.Error())
	if ev2, ok := status.FromError(e2); ok {
		require.Equal(t, ev2.Code(), e3.Code())
	}
}
