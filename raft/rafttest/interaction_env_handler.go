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
)

// Handle is the entrypoint for data-driven interaction testing. Commands and
// parameters are parsed from the supplied TestData. Errors during data parsing
// are reported via the supplied *testing.T; errors from the raft nodes and the
// storage engine are reported to the output buffer.
func (env *InteractionEnv) Handle(t *testing.T, d datadriven.TestData) string {
	env.Output.Reset()
	var err error
	switch d.Cmd {
	case "_breakpoint":
		// This is a helper case to attach a debugger to when a problem needs
		// to be investigated in a longer test file. In such a case, add the
		// following stanza immediately before the interesting behavior starts:
		//
		// _breakpoint:
		// ----
		// ok
		//
		// and set a breakpoint on the `case` above.
	case "add-nodes":
		// Example:
		//
		// add-nodes <number-of-nodes-to-add> voters=(1 2 3) learners=(4 5) index=2 content=foo
		err = env.handleAddNodes(t, d)
	case "campaign":
		// Example:
		//
		// campaign <id-of-candidate>
		err = env.handleCampaign(t, d)
	case "compact":
		// Example:
		//
		// compact <id> <new-first-index>
		err = env.handleCompact(t, d)
	case "deliver-msgs":
		// Deliver the messages for a given recipient.
		//
		// Example:
		//
		// deliver-msgs <idx>
		err = env.handleDeliverMsgs(t, d)
	case "process-ready":
		// Example:
		//
		// process-ready 3
		err = env.handleProcessReady(t, d)
	case "log-level":
		// Set the log level. NONE disables all output, including from the test
		// harness (except errors).
		//
		// Example:
		//
		// log-level WARN
		err = env.handleLogLevel(t, d)
	case "raft-log":
		// Print the Raft log.
		//
		// Example:
		//
		// raft-log 3
		err = env.handleRaftLog(t, d)
	case "stabilize":
		// Deliver messages to and run process-ready on the set of IDs until
		// no more work is to be done.
		//
		// Example:
		//
		// stabilize 1 4
		err = env.handleStabilize(t, d)
	case "status":
		// Print Raft status.
		//
		// Example:
		//
		// status 5
		err = env.handleStatus(t, d)
	case "tick-heartbeat":
		// Tick a heartbeat interval.
		//
		// Example:
		//
		// tick-heartbeat 3
		err = env.handleTickHeartbeat(t, d)
	case "propose":
		// Propose an entry.
		//
		// Example:
		//
		// propose 1 foo
		err = env.handlePropose(t, d)
	case "propose-conf-change":
		// Propose a configuration change.
		//
		// Example:
		//
		// propose-conf-change transition=explicit
		// v1 v3 l4 r5
		//
		// Example:
		//
		// propose-conf-change v1=true
		// v5
		err = env.handleProposeConfChange(t, d)
	default:
		err = fmt.Errorf("unknown command")
	}
	if err != nil {
		env.Output.WriteString(err.Error())
	}
	// NB: the highest log level suppresses all output, including that of the
	// handlers. This comes in useful during setup which can be chatty.
	// However, errors are always logged.
	if env.Output.Len() == 0 {
		return "ok"
	}
	if env.Output.Lvl == len(lvlNames)-1 {
		if err != nil {
			return err.Error()
		}
		return "ok (quiet)"
	}
	return env.Output.String()
}

func firstAsInt(t *testing.T, d datadriven.TestData) int {
	t.Helper()
	n, err := strconv.Atoi(d.CmdArgs[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func firstAsNodeIdx(t *testing.T, d datadriven.TestData) int {
	t.Helper()
	n := firstAsInt(t, d)
	return n - 1
}

func nodeIdxs(t *testing.T, d datadriven.TestData) []int {
	var ints []int
	for i := 0; i < len(d.CmdArgs); i++ {
		if len(d.CmdArgs[i].Vals) != 0 {
			continue
		}
		n, err := strconv.Atoi(d.CmdArgs[i].Key)
		if err != nil {
			t.Fatal(err)
		}
		ints = append(ints, n-1)
	}
	return ints
}
