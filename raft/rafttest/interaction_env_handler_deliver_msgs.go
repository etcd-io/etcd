// Copyright 2019 The etcd Authors
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

package rafttest

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cockroachdb/datadriven"
	"go.etcd.io/etcd/raft"
	"go.etcd.io/etcd/raft/raftpb"
)

func (env *InteractionEnv) handleDeliverMsgs(t *testing.T, d datadriven.TestData) error {
	if len(env.Messages) == 0 {
		return errors.New("no messages to deliver")
	}

	msgs := env.Messages
	env.Messages = nil

	return env.DeliverMsgs(msgs)
}

// DeliverMsgs delivers the supplied messages typically taken from env.Messages.
func (env *InteractionEnv) DeliverMsgs(msgs []raftpb.Message) error {
	for _, msg := range msgs {
		toIdx := int(msg.To - 1)
		var drop bool
		if toIdx >= len(env.Nodes) {
			// Drop messages for peers that don't exist yet.
			drop = true
			env.Output.WriteString("dropped: ")
		}
		fmt.Fprintln(env.Output, raft.DescribeMessage(msg, defaultEntryFormatter))
		if drop {
			continue
		}
		if err := env.Nodes[toIdx].Step(msg); err != nil {
			env.Output.WriteString(err.Error())
			continue
		}
	}
	return nil
}
