package etcdhttp

import (
	"bytes"
	"log"
	"net/http"

	"github.com/coreos/etcd/elog"
	"github.com/coreos/etcd/raft/raftpb"
)

func Sender(p Cluster) func(msgs []raftpb.Message) {
	return func(msgs []raftpb.Message) {
		for _, m := range msgs {
			// TODO: reuse go routines
			// limit the number of outgoing connections for the same receiver
			go send(p, m)
		}
	}
}

func send(p Cluster, m raftpb.Message) {
	// TODO (xiangli): reasonable retry logic
	for i := 0; i < 3; i++ {
		url := p.Pick(m.To)
		if url == "" {
			// TODO: unknown peer id.. what do we do? I
			// don't think his should ever happen, need to
			// look into this further.
			log.Printf("etcdhttp: no addr for %d", m.To)
			return
		}

		url += raftPrefix

		// TODO: don't block. we should be able to have 1000s
		// of messages out at a time.
		data, err := m.Marshal()
		if err != nil {
			log.Println("etcdhttp: dropping message:", err)
			return // drop bad message
		}
		if httpPost(url, data) {
			return // success
		}
		// TODO: backoff
	}
}

func httpPost(url string, data []byte) bool {
	// TODO: set timeouts
	resp, err := http.Post(url, "application/protobuf", bytes.NewBuffer(data))
	if err != nil {
		elog.TODO()
		return false
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		elog.TODO()
		return false
	}
	return true
}
