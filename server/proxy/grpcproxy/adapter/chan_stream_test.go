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

package adapter

import (
	"context"
	"errors"
	"io"
	"testing"
)

// TestPipeStream_HandlerSuccessReturnsEOF verifies that when a server-streaming
// handler completes successfully (returns nil), the client receives io.EOF from
// RecvMsg. This matches the gRPC ClientStream contract:
//
//	"It returns io.EOF when the stream completes successfully."
//	https://github.com/grpc/grpc-go/blob/master/stream.go
func TestPipeStream_HandlerSuccessReturnsEOF(t *testing.T) {
	cs := newPipeStream(context.Background(), func(ss chanServerStream) error {
		ss.SendMsg("msg1") //nolint:staticcheck
		ss.SendMsg("msg2") //nolint:staticcheck
		return nil
	})

	var v any

	if err := cs.RecvMsg(&v); err != nil {
		t.Fatalf("RecvMsg() = %v, want nil", err)
	}
	if v != "msg1" {
		t.Fatalf("RecvMsg() got %v, want msg1", v)
	}

	if err := cs.RecvMsg(&v); err != nil {
		t.Fatalf("RecvMsg() = %v, want nil", err)
	}
	if v != "msg2" {
		t.Fatalf("RecvMsg() got %v, want msg2", v)
	}

	if err := cs.RecvMsg(&v); !errors.Is(err, io.EOF) {
		t.Fatalf("RecvMsg() = %v, want io.EOF (per gRPC ClientStream contract)", err)
	}
}

// TestPipeStream_HandlerErrorPropagated verifies that when a handler returns a
// non-nil error, the client receives that exact error from RecvMsg.
func TestPipeStream_HandlerErrorPropagated(t *testing.T) {
	handlerErr := errors.New("handler failed")
	cs := newPipeStream(context.Background(), func(ss chanServerStream) error {
		ss.SendMsg("partial") //nolint:staticcheck
		return handlerErr
	})

	var v any

	if err := cs.RecvMsg(&v); err != nil {
		t.Fatalf("RecvMsg() = %v, want nil", err)
	}
	if v != "partial" {
		t.Fatalf("RecvMsg() got %v, want partial", v)
	}

	if err := cs.RecvMsg(&v); !errors.Is(err, handlerErr) {
		t.Fatalf("RecvMsg() = %v, want %v", err, handlerErr)
	}
}
