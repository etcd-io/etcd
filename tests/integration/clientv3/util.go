// Copyright 2017 The etcd Authors
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

package clientv3test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MustWaitPinReady waits up to 3-second until connection is up (pin endpoint).
// Fatal on time-out.
func MustWaitPinReady(t *testing.T, cli *clientv3.Client) {
	// TODO: decrease timeout after balancer rewrite!!!
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, err := cli.Get(ctx, "foo")
	cancel()
	if err != nil {
		t.Fatal(err)
	}
}

// IsServerCtxTimeout checks reason of the error.
// e.g. due to clock drifts in server-side,
// client context times out first in server-side
// while original client-side context is not timed out yet
func IsServerCtxTimeout(err error) bool {
	if err == nil {
		return false
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.DeadlineExceeded && strings.Contains(err.Error(), "context deadline exceeded")
}

// IsClientTimeout checks reason of the error.
// In grpc v1.11.3+ dial timeouts can error out with transport.ErrConnClosing. Previously dial timeouts
// would always error out with context.DeadlineExceeded.
func IsClientTimeout(err error) bool {
	if err == nil {
		return false
	}
	if err == context.DeadlineExceeded {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.DeadlineExceeded
}

func IsCanceled(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.Canceled
}

func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled {
		return true
	}
	ev, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := ev.Code()
	return code == codes.Unavailable
}
