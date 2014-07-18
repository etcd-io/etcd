/*
Copyright 2014 CoreOS Inc.

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

package etcd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coreos/etcd/raft"
)

type v2Proposal struct {
	data []byte
	ret  chan interface{}
}

type wait struct {
	index int64
	term  int64
}

type v2Raft struct {
	*raft.Node
	result map[wait]chan interface{}
	term   int64
}

func (r *v2Raft) Propose(p v2Proposal) {
	if !r.Node.IsLeader() {
		p.ret <- fmt.Errorf("not leader")
		return
	}
	r.Node.Propose(p.data)
	r.result[wait{r.Index(), r.Term()}] = p.ret
	return
}

func (r *v2Raft) Sync() {
	if !r.Node.IsLeader() {
		return
	}
	sync := &cmd{Type: "sync", Time: time.Now()}
	data, err := json.Marshal(sync)
	if err != nil {
		panic(err)
	}
	r.Node.Propose(data)
}

func (r *v2Raft) StopProposalWaiters() {
	for k, ch := range r.result {
		ch <- raftStopErr
		delete(r.result, k)
	}
}
