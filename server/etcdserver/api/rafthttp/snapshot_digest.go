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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"net/http"
)

// snapshotDigestTrailer covers the DB payload, not the preceding Raft envelope.
const snapshotDigestTrailer = "X-Etcd-Snapshot-Sha256"

var errSnapshotDigest = errors.New("rafthttp: database snapshot digest does not match")

// The digest is made available only after a clean end of the source stream.
type snapshotDigestReader struct {
	reader io.Reader
	hash   hash.Hash
	finish func(string) error
	end    error
}

func newSnapshotDigestReader(r io.Reader, finish func(string) error) *snapshotDigestReader {
	return &snapshotDigestReader{reader: r, hash: sha256.New(), finish: finish}
}

func (r *snapshotDigestReader) Read(p []byte) (int, error) {
	if r.end != nil {
		return 0, r.end
	}
	n, err := r.reader.Read(p)
	_, _ = r.hash.Write(p[:n])
	if err == io.EOF {
		if verifyErr := r.finish(hex.EncodeToString(r.hash.Sum(nil))); verifyErr != nil {
			err = verifyErr
		}
	}
	r.end = err
	return n, err
}

func snapshotPayloadReader(req *http.Request) io.Reader {
	if _, declared := req.Trailer[snapshotDigestTrailer]; !declared {
		return req.Body
	}
	return newSnapshotDigestReader(req.Body, func(digest string) error {
		values := req.Trailer.Values(snapshotDigestTrailer)
		if len(values) != 1 || values[0] != digest {
			return errSnapshotDigest
		}
		return nil
	})
}
