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

package raft

import (
	pb "github.com/coreos/etcd/raft/raftpb"
)

type Status struct {
	ID uint64

	pb.HardState
	SoftState

	Applied  uint64
	Progress map[uint64]progress
}

func getStatus(r *raft) Status {
	s := Status{ID: r.id}
	s.HardState = r.HardState
	s.SoftState = *r.softState()

	s.Applied = r.raftLog.applied

	if s.RaftState == StateLeader {
		s.Progress = make(map[uint64]progress)
		for id, p := range r.prs {
			s.Progress[id] = *p
		}
	}

	return s
}
