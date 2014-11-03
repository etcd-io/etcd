/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

/*
Package raft provides an implementation of the raft consensus algorithm.

The primary object in raft is a Node. You either start a Node from scratch
using raft.StartNode or start a Node from some initial state using raft.RestartNode.

	n := raft.StartNode(0x01, []int64{0x02, 0x03}, 3, 1)

Now that you are holding onto a Node you have a few responsibilities:

First, you need to push messages that you receive from other machines into the
Node with n.Step().

	func recvRaftRPC(ctx context.Context, m raftpb.Message) {
		n.Step(ctx, m)
	}

Second, you need to save log entries to storage, process committed log entries
through your application and then send pending messages to peers by reading the
channel returned by n.Ready(). It is important that the user persist any
entries that require stable storage before sending messages to other peers to
ensure fault-tolerance.

And finally you need to service timeouts with Tick(). Raft has two important
timeouts: heartbeat and the election timeout. However, internally to the raft
package time is represented by an abstract "tick". The user is responsible for
calling Tick() on their raft.Node on a regular interval in order to service
these timeouts.

The total state machine handling loop will look something like this:

	for {
		select {
		case <-s.Ticker:
			n.Tick()
		case rd := <-s.Node.Ready():
			saveToStable(rd.State, rd.Entries)
			send(rd.Messages)
			process(rd.CommittedEntries)
			s.Node.Advance()
		case <-s.done:
			return
		}
	}

To propose changes to the state machine from your node take your application
data, serialize it into a byte slice and call:

	n.Propose(ctx, data)

If the proposal is committed, data will appear in committed entries with type
raftpb.EntryNormal.

To add or remove node in a cluster, build ConfChange struct 'cc' and call:

	n.ProposeConfChange(ctx, cc)

After config change is committed, some committed entry with type
raftpb.EntryConfChange will be returned. You should apply it to node through:

	var cc raftpb.ConfChange
	cc.Unmarshal(data)
	n.ApplyConfChange(cc)

Note: An ID represents a unique node in a cluster. A given ID MUST be used
only once even if the old node has been removed.

*/
package raft
