// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build integration

package pubsub

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/internal/testutil"
)

func TestAll(t *testing.T) {
	ctx := testutil.Context(ScopePubSub, ScopeCloudPlatform)
	now := time.Now()
	topic := fmt.Sprintf("topic-%d", now.Unix())
	subscription := fmt.Sprintf("subscription-%d", now.Unix())

	if err := CreateTopic(ctx, topic); err != nil {
		t.Errorf("CreateTopic error: %v", err)
	}

	if err := CreateSub(ctx, subscription, topic, time.Duration(0), ""); err != nil {
		t.Errorf("CreateSub error: %v", err)
	}

	exists, err := TopicExists(ctx, topic)
	if err != nil {
		t.Errorf("TopicExists error: %v", err)
	}
	if !exists {
		t.Errorf("topic %s should exist, but it doesn't", topic)
	}

	exists, err = SubExists(ctx, subscription)
	if err != nil {
		t.Errorf("SubExists error: %v", err)
	}
	if !exists {
		t.Errorf("subscription %s should exist, but it doesn't", subscription)
	}

	max := 10
	msgs := make([]*Message, max)
	expectedMsgs := make(map[string]bool, max)
	for i := 0; i < max; i++ {
		text := fmt.Sprintf("a message with an index %d", i)
		labels := make(map[string]string)
		labels["foo"] = "bar"
		msgs[i] = &Message{
			Data:   []byte(text),
			Labels: labels,
		}
		expectedMsgs[text] = false
	}

	ids, err := Publish(ctx, topic, msgs...)
	if err != nil {
		t.Errorf("Publish (1) error: %v", err)
	}
	if len(ids) != max {
		t.Errorf("unexpected number of message IDs received; %d, want %d", len(ids), max)
	}
	expectedIDs := make(map[string]bool, max)
	for _, id := range ids {
		expectedIDs[id] = false
	}

	received, err := PullWait(ctx, subscription, max)
	if err != nil {
		t.Errorf("PullWait error: %v", err)
	}
	if len(received) != max {
		t.Errorf("unexpected number of messages received; %d, want %d", len(received), max)
	}
	for _, msg := range received {
		expectedMsgs[string(msg.Data)] = true
		expectedIDs[msg.ID] = true
		if msg.Labels["foo"] != "bar" {
			t.Errorf("message label foo is expected to be 'bar', found '%s'", msg.Labels["foo"])
		}
	}
	for msg, found := range expectedMsgs {
		if !found {
			t.Errorf("message '%s' should be received", msg)
		}
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("message with the message id '%s' should be received", id)
		}
	}

	err = DeleteSub(ctx, subscription)
	if err != nil {
		t.Errorf("DeleteSub error: %v", err)
	}

	err = DeleteTopic(ctx, topic)
	if err != nil {
		t.Errorf("DeleteTopic error: %v", err)
	}
}
