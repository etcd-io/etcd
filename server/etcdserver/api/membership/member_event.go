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

package membership

import (
	"sync"
)

// MemberEventType represents the type of membership change event.
type MemberEventType int

const (
	// MemberAdded indicates a new member was added to the cluster.
	MemberAdded MemberEventType = iota
	// MemberRemoved indicates a member was removed from the cluster.
	MemberRemoved
	// MemberUpdated indicates a member's configuration was updated.
	MemberUpdated
	// MemberPromoted indicates a learner member was promoted to voting member.
	MemberPromoted
)

// MemberEvent represents a membership change event.
type MemberEvent struct {
	Type   MemberEventType
	Member *Member
}

// MemberEventBroadcaster broadcasts membership events to multiple subscribers.
type MemberEventBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[int64]chan MemberEvent
	nextID      int64
}

// NewMemberEventBroadcaster creates a new MemberEventBroadcaster.
func NewMemberEventBroadcaster() *MemberEventBroadcaster {
	return &MemberEventBroadcaster{
		subscribers: make(map[int64]chan MemberEvent),
	}
}

// Subscribe creates a new subscription for membership events.
// Returns a subscription ID and a channel to receive events.
// The channel has a buffer to prevent blocking the broadcaster.
func (b *MemberEventBroadcaster) Subscribe() (int64, <-chan MemberEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++

	ch := make(chan MemberEvent, 64)
	b.subscribers[id] = ch

	return id, ch
}

// Unsubscribe removes a subscription.
func (b *MemberEventBroadcaster) Unsubscribe(id int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if ch, ok := b.subscribers[id]; ok {
		close(ch)
		delete(b.subscribers, id)
	}
}

// Broadcast sends an event to all subscribers.
// Events are sent non-blocking; if a subscriber's channel is full, the event is dropped for that subscriber.
func (b *MemberEventBroadcaster) Broadcast(event MemberEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Channel is full, drop the event for this subscriber.
			// This prevents slow subscribers from blocking the broadcaster.
		}
	}
}

// SubscriberCount returns the number of active subscribers.
func (b *MemberEventBroadcaster) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}
