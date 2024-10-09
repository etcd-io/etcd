// Copyright 2022 The etcd Authors
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

package testutils

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
)

func ExecuteWithTimeout(t *testing.T, timeout time.Duration, f func()) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ExecuteUntil(ctx, t, f)
}

func ExecuteUntil(ctx context.Context, t *testing.T, f func()) {
	deadline, deadlineSet := ctx.Deadline()
	timeout := time.Until(deadline)
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		f()
	}()

	select {
	case <-ctx.Done():
		msg := ctx.Err().Error()
		if deadlineSet {
			msg = fmt.Sprintf("test timed out after %v, err: %v", timeout, msg)
		}
		testutil.FatalStack(t, msg)
	case <-donec:
	}
}
