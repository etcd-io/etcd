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

package etcdserver

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/raft/raftpb"
)

const raftPrefix = "/raft"

// Sender creates the default production sender used to transport raft messages
// in the cluster. The returned sender will update the given ServerStats and
// LeaderStats appropriately.
func Sender(t *http.Transport, cl *Cluster, ss *stats.ServerStats, ls *stats.LeaderStats) func(msgs []raftpb.Message) {
	c := &http.Client{Transport: t}

	return func(msgs []raftpb.Message) {
		for _, m := range msgs {
			// TODO: reuse go routines
			// limit the number of outgoing connections for the same receiver
			go send(c, cl, m, ss, ls)
		}
	}
}

// send uses the given client to send a message to a member in the given
// ClusterStore, retrying up to 3 times for each message. The given
// ServerStats and LeaderStats are updated appropriately
func send(c *http.Client, cl *Cluster, m raftpb.Message, ss *stats.ServerStats, ls *stats.LeaderStats) {
	cid := cl.ID()
	// TODO (xiangli): reasonable retry logic
	for i := 0; i < 3; i++ {
		memb := cl.Member(m.To)
		if memb == nil {
			// TODO: unknown peer id.. what do we do? I
			// don't think his should ever happen, need to
			// look into this further.
			log.Printf("etcdhttp: no member for %d", m.To)
			return
		}
		u := fmt.Sprintf("%s%s", memb.Pick(), raftPrefix)

		// TODO: don't block. we should be able to have 1000s
		// of messages out at a time.
		data, err := m.Marshal()
		if err != nil {
			log.Println("etcdhttp: dropping message:", err)
			return // drop bad message
		}
		if m.Type == raftpb.MsgApp {
			ss.SendAppendReq(len(data))
		}
		to := idAsHex(m.To)
		fs := ls.Follower(to)

		start := time.Now()
		sent := httpPost(c, u, cid, data)
		end := time.Now()
		if sent {
			fs.Succ(end.Sub(start))
			return
		}
		fs.Fail()
		// TODO: backoff
	}
}

// httpPost POSTs a data payload to a url using the given client. Returns true
// if the POST succeeds, false on any failure.
func httpPost(c *http.Client, url string, cid uint64, data []byte) bool {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		// TODO: log the error?
		return false
	}
	req.Header.Set("Content-Type", "application/protobuf")
	req.Header.Set("X-Etcd-Cluster-ID", strconv.FormatUint(cid, 16))
	resp, err := c.Do(req)
	if err != nil {
		// TODO: log the error?
		return false
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusPreconditionFailed:
		// TODO: shutdown the etcdserver gracefully?
		log.Panicf("clusterID mismatch")
		return false
	case http.StatusForbidden:
		// TODO: stop the server
		log.Panicf("the member has been removed")
		return false
	case http.StatusNoContent:
		return true
	default:
		return false
	}
}
