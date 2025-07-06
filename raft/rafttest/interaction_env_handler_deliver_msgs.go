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
	"fmt"
	"strconv"
	"testing"

	"github.com/cockroachdb/datadriven"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func (env *InteractionEnv) handleDeliverMsgs(t *testing.T, d datadriven.TestData) error {
	var rs []Recipient
	for _, arg := range d.CmdArgs {
		if len(arg.Vals) == 0 {
			id, err := strconv.ParseUint(arg.Key, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			rs = append(rs, Recipient{ID: id})
		}
		for i := range arg.Vals {
			switch arg.Key {
			case "drop":
				var id uint64
				arg.Scan(t, i, &id)
				var found bool
				for _, r := range rs {
					if r.ID == id {
						found = true
					}
				}
				if found {
					t.Fatalf("can't both deliver and drop msgs to %d", id)
				}
				rs = append(rs, Recipient{ID: id, Drop: true})
			}
		}
	}

	if n := env.DeliverMsgs(rs...); n == 0 {
		env.Output.WriteString("no messages\n")
	}
	return nil
}

type Recipient struct {
	ID   uint64
	Drop bool
}

// DeliverMsgs goes through env.Messages and, depending on the Drop flag,
// delivers or drops messages to the specified Recipients. Returns the
// number of messages handled (i.e. delivered or dropped). A handled message
// is removed from env.Messages.
func (env *InteractionEnv) DeliverMsgs(rs ...Recipient) int {
	var n int
	for _, r := range rs {
		var msgs []raftpb.Message
		msgs, env.Messages = splitMsgs(env.Messages, r.ID)
		n += len(msgs)
		for _, msg := range msgs {
			if r.Drop {
				fmt.Fprint(env.Output, "dropped: ")
			}
			fmt.Fprintln(env.Output, raft.DescribeMessage(msg, defaultEntryFormatter))
			if r.Drop {
				// NB: it's allowed to drop messages to nodes that haven't been instantiated yet,
				// we haven't used msg.To yet.
				continue
			}
			toIdx := int(msg.To - 1)
			if err := env.Nodes[toIdx].Step(msg); err != nil {
				fmt.Fprintln(env.Output, err)
			}
		}
	}
	return n
}
