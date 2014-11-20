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

package rafthttp

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/etcd/pkg/pbutil"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/raft/raftpb"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

func TestServeRaft(t *testing.T) {
	testCases := []struct {
		method    string
		body      io.Reader
		p         Processor
		clusterID string

		wcode int
	}{
		{
			// bad method
			"GET",
			bytes.NewReader(
				pbutil.MustMarshal(&raftpb.Message{}),
			),
			&nopProcessor{},
			"0",
			http.StatusMethodNotAllowed,
		},
		{
			// bad method
			"PUT",
			bytes.NewReader(
				pbutil.MustMarshal(&raftpb.Message{}),
			),
			&nopProcessor{},
			"0",
			http.StatusMethodNotAllowed,
		},
		{
			// bad method
			"DELETE",
			bytes.NewReader(
				pbutil.MustMarshal(&raftpb.Message{}),
			),
			&nopProcessor{},
			"0",
			http.StatusMethodNotAllowed,
		},
		{
			// bad request body
			"POST",
			&errReader{},
			&nopProcessor{},
			"0",
			http.StatusBadRequest,
		},
		{
			// bad request protobuf
			"POST",
			strings.NewReader("malformed garbage"),
			&nopProcessor{},
			"0",
			http.StatusBadRequest,
		},
		{
			// good request, wrong cluster ID
			"POST",
			bytes.NewReader(
				pbutil.MustMarshal(&raftpb.Message{}),
			),
			&nopProcessor{},
			"1",
			http.StatusPreconditionFailed,
		},
		{
			// good request, Processor failure
			"POST",
			bytes.NewReader(
				pbutil.MustMarshal(&raftpb.Message{}),
			),
			&errProcessor{
				err: &resWriterToError{code: http.StatusForbidden},
			},
			"0",
			http.StatusForbidden,
		},
		{
			// good request, Processor failure
			"POST",
			bytes.NewReader(
				pbutil.MustMarshal(&raftpb.Message{}),
			),
			&errProcessor{
				err: &resWriterToError{code: http.StatusInternalServerError},
			},
			"0",
			http.StatusInternalServerError,
		},
		{
			// good request, Processor failure
			"POST",
			bytes.NewReader(
				pbutil.MustMarshal(&raftpb.Message{}),
			),
			&errProcessor{err: errors.New("blah")},
			"0",
			http.StatusInternalServerError,
		},
		{
			// good request
			"POST",
			bytes.NewReader(
				pbutil.MustMarshal(&raftpb.Message{}),
			),
			&nopProcessor{},
			"0",
			http.StatusNoContent,
		},
	}
	for i, tt := range testCases {
		req, err := http.NewRequest(tt.method, "foo", tt.body)
		if err != nil {
			t.Fatalf("#%d: could not create request: %#v", i, err)
		}
		req.Header.Set("X-Etcd-Cluster-ID", tt.clusterID)
		rw := httptest.NewRecorder()
		h := NewHandler(tt.p, types.ID(0))
		h.ServeHTTP(rw, req)
		if rw.Code != tt.wcode {
			t.Errorf("#%d: got code=%d, want %d", i, rw.Code, tt.wcode)
		}
	}
}

// errReader implements io.Reader to facilitate a broken request.
type errReader struct{}

func (er *errReader) Read(_ []byte) (int, error) { return 0, errors.New("some error") }

type nopProcessor struct{}

func (p *nopProcessor) Process(ctx context.Context, m raftpb.Message) error { return nil }

type errProcessor struct {
	err error
}

func (p *errProcessor) Process(ctx context.Context, m raftpb.Message) error { return p.err }

type resWriterToError struct {
	code int
}

func (e *resWriterToError) Error() string                 { return "" }
func (e *resWriterToError) WriteTo(w http.ResponseWriter) { w.WriteHeader(e.code) }
