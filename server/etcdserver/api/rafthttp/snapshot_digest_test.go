// Copyright 2015 The etcd Authors
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

package rafthttp

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/raft/v3/raftpb"
)

func TestSnapshotDigestValidPeerStreams(t *testing.T) {
	db, err := bbolt.Open(filepath.Join(t.TempDir(), "fixture.db"), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucket([]byte("fixture"))
		if err != nil {
			return err
		}
		return bucket.Put([]byte("key"), []byte("value"))
	}); err != nil {
		t.Fatal(err)
	}
	var snapshot bytes.Buffer
	if err := db.View(func(tx *bbolt.Tx) error {
		_, err := tx.WriteTo(&snapshot)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name        string
		enableHTTP2 bool
		legacyFirst bool
	}{
		{name: "HTTP1/digest first"},
		{name: "HTTP1/legacy first", legacyFirst: true},
		{name: "HTTP2/digest first", enableHTTP2: true},
		{name: "HTTP2/legacy first", enableHTTP2: true, legacyFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			snapshots := snap.New(lg, t.TempDir())
			r := &fakeRaft{}
			tr := &Transport{ClusterID: types.ID(1), Raft: r}
			h := newSnapshotHandler(tr, r, snapshots, types.ID(1))
			type observation struct {
				proto  int
				digest string
			}
			observed := make(chan observation, 2)
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				h.ServeHTTP(w, req)
				observed <- observation{req.ProtoMajor, req.Trailer.Get(snapshotDigestTrailer)}
			}))
			wantProto := 1
			if tc.enableHTTP2 {
				srv.EnableHTTP2 = true
				srv.StartTLS()
				wantProto = 2
			} else {
				srv.Start()
			}
			defer srv.Close()
			tr.pipelineRt = srv.Client().Transport

			picker := mustNewURLPicker(t, []string{srv.URL})
			sender := newSnapshotSender(tr, picker, types.ID(1), newPeerStatus(lg, types.ID(0), types.ID(1)))
			defer sender.stop()

			const idx = uint64(1)
			var first os.FileInfo
			for _, legacy := range []bool{tc.legacyFirst, !tc.legacyFirst} {
				m := &raftpb.Message{
					Type: raftpb.MsgSnap.Enum(),
					To:   new(uint64(1)),
					Snapshot: &raftpb.Snapshot{
						Metadata: &raftpb.SnapshotMetadata{Index: new(uint64(idx))},
					},
				}
				sm := snap.NewMessage(m, io.NopCloser(bytes.NewReader(snapshot.Bytes())), int64(snapshot.Len()))
				if legacy {
					sendLegacySnapshot(t, lg, srv, sm)
				} else {
					sender.send(sm)
					if !<-sm.CloseNotify() {
						t.Fatal("the snapshot sender did not confirm success")
					}
				}

				wantDigest := fmt.Sprintf("%x", sha256.Sum256(snapshot.Bytes()))
				if legacy {
					wantDigest = ""
				}
				got := <-observed
				if got.proto != wantProto {
					t.Fatalf("HTTP protocol major = %d, want %d", got.proto, wantProto)
				}
				if got.digest != wantDigest {
					t.Fatalf("snapshot digest = %q, want %q", got.digest, wantDigest)
				}
				path, err := snapshots.DBFilePath(idx)
				if err != nil {
					t.Fatal(err)
				}
				file, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if first != nil && !os.SameFile(first, file) {
					t.Fatal("a successful duplicate stream replaced the existing file")
				}
				first = file
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(body, snapshot.Bytes()) {
					t.Fatal("saved snapshot differs from the valid source snapshot")
				}
			}
		})
	}
}

func sendLegacySnapshot(t *testing.T, lg *zap.Logger, srv *httptest.Server, sm *snap.Message) {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	body := createSnapBody(lg, sm, nil)
	defer body.Close()
	req := createPostRequest(lg, *u, RaftSnapshotPrefix, body, "application/octet-stream", nil, 0, 1)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("legacy snapshot reply = %s", resp.Status)
	}
}
