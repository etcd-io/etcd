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

package cmd

import (
	"context"
	"fmt"
	"sync"

	v3 "go.etcd.io/etcd/client/v3"
)

type casPutRequest struct {
	key   string
	value string
}

type casRevisionTracker struct {
	mu        sync.Mutex
	revisions map[string]int64
}

func newCASRevisionTracker() *casRevisionTracker {
	return &casRevisionTracker{revisions: make(map[string]int64)}
}

func (t *casRevisionTracker) put(ctx context.Context, c *v3.Client, key string, value string) error {
	for {
		expectedRevision := t.revision(key)
		newRevision, observedRevision, succeeded, err := casPutAttempt(ctx, c, key, value, expectedRevision)
		if err != nil {
			return err
		}
		if succeeded {
			t.updateIfNewer(key, newRevision)
			return nil
		}
		t.update(key, observedRevision)
	}
}

func (t *casRevisionTracker) revision(key string) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.revisions[key]
}

func (t *casRevisionTracker) update(key string, revision int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.revisions[key] = revision
}

func (t *casRevisionTracker) updateIfNewer(key string, revision int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if revision > t.revisions[key] {
		t.revisions[key] = revision
	}
}

func casPutSingle(ctx context.Context, c *v3.Client, key string, value string, expectedRevision *int64) error {
	if expectedRevision == nil {
		return fmt.Errorf("expectedRevision must not be nil")
	}
	for {
		newRevision, observedRevision, succeeded, err := casPutAttempt(ctx, c, key, value, *expectedRevision)
		if err != nil {
			return err
		}
		if succeeded {
			*expectedRevision = newRevision
			return nil
		}
		*expectedRevision = observedRevision
	}
}

func casPutAttempt(ctx context.Context, c *v3.Client, key string, value string, expectedRevision int64) (newRevision int64, observedRevision int64, succeeded bool, err error) {
	txnResp, err := c.Txn(ctx).
		If(v3.Compare(v3.ModRevision(key), "=", expectedRevision)).
		Then(v3.OpPut(key, value)).
		Else(v3.OpGet(key)).
		Commit()
	if err != nil {
		return 0, 0, false, err
	}
	if txnResp.Succeeded {
		return txnResp.Header.Revision, 0, true, nil
	}

	revisions, err := txnPutObservedModRevisions(txnResp, 1)
	if err != nil {
		return 0, 0, false, err
	}
	return 0, revisions[0], false, nil
}
