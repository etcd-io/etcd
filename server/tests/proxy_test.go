package server

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	// "time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/coreos/raft"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensures that a server will stream committed entries.
//
//   $ curl -X PUT localhost:4001/v2/keys/foo -d value=XXX
//   $ curl -X PUT localhost:4001/v2/keys/bar -d value=XXX
//   $ curl localhost:7001/log/committed
//
func TestPeerCommittedLog(t *testing.T) {
	tests.RunServer(func(s *server.Server) {
		// Create some entries on the log.
		tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/foo"), url.Values{"value":{"XXX"}})
		tests.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/bar"), url.Values{"value":{"YYY"}})
		
		// Retrieve committed entries.
		transport := &http.Transport{DisableKeepAlives: true}
		req, _ := http.NewRequest("GET", "http://localhost:7701/log/committed", nil)
		req.Header.Set("Content-Type", "application/json")
		c := &http.Client{Transport: transport}
		resp, _ := c.Do(req)
		assert.NotNil(t, resp)

		// Close request and check contents.
		transport.CancelRequest(req)
		assert.NotNil(t, resp)
		
		var typ uint8
		var size uint64
		var entry raft.LogEntry
		binary.Read(resp.Body, binary.BigEndian, &typ)
		binary.Read(resp.Body, binary.BigEndian, &size)
		assert.Equal(t, typ, uint8(1))
		assert.Equal(t, size, uint64(0x90))
		_, err := entry.Decode(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, entry.CommandName(), "etcd:join")
		assert.Equal(t, string(entry.Command()), `{"minVersion":2,"maxVersion":2,"name":"ETCDTEST","raftURL":"http://localhost:7701","etcdURL":"http://localhost:4401"}`+"\n")
		resp.Body.Close()
	})
}
