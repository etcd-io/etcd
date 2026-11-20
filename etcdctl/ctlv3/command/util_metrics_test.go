// Copyright 2026 The etcd Authors
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

package command

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type metricsCountingBody struct {
	io.ReadCloser
	bytesRead int
	sawEOF    bool
	closed    bool
}

func (b *metricsCountingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.bytesRead += n
	if err == io.EOF {
		b.sawEOF = true
	}
	return n, err
}

func (b *metricsCountingBody) Close() error {
	b.closed = true
	return b.ReadCloser.Close()
}

type metricsCountingTransport struct {
	http.RoundTripper
	body *metricsCountingBody
}

func (tr *metricsCountingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := tr.RoundTripper.RoundTrip(req)
	if err == nil {
		tr.body = &metricsCountingBody{ReadCloser: resp.Body}
		resp.Body = tr.body
	}
	return resp, err
}

func TestEndpointMemoryMetrics(t *testing.T) {
	const limit = 8 << 20
	tests := []struct {
		name      string
		bodySize  int
		wantValue float64
	}{
		{name: "small", bodySize: 50, wantValue: 1234},
		{name: "two_mebibytes", bodySize: (2 << 20) + 34, wantValue: 1234},
		{name: "just_under_limit", bodySize: limit - 1, wantValue: 1234},
		{name: "at_limit", bodySize: limit, wantValue: 1234},
		{name: "one_byte_over_limit", bodySize: limit + 1, wantValue: 0},
		{name: "well_over_limit", bodySize: 2 * limit, wantValue: 0},
	}

	// These subtests replace http.DefaultTransport and os.Stdout and must run serially.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const metric = "process_resident_memory_bytes 1234\n"
			body := metric + "#" + strings.Repeat("x", tt.bodySize-len(metric)-2) + "\n"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/metrics" {
					t.Errorf("unexpected request path: %s", r.URL.Path)
				}
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()

			previousTransport := http.DefaultTransport
			transport := previousTransport.(*http.Transport).Clone()
			defer transport.CloseIdleConnections()
			counter := &metricsCountingTransport{RoundTripper: transport}
			http.DefaultTransport = counter
			defer func() { http.DefaultTransport = previousTransport }()

			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			defer writer.Close()
			previousStdout := os.Stdout
			os.Stdout = writer
			defer func() { os.Stdout = previousStdout }()

			got := endpointMemoryMetrics(server.URL, nil)
			os.Stdout = previousStdout
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			output, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if counter.body == nil {
				t.Fatalf("no response body read; output: %s", output)
			}
			t.Logf("body bytes=%d; client bytes read=%d; reached EOF=%t; memory=%g; output=%q", len(body), counter.body.bytesRead, counter.body.sawEOF, got, output)
			if got != tt.wantValue {
				t.Errorf("endpointMemoryMetrics() = %g, want %g", got, tt.wantValue)
			}
			wantOutput := ""
			if tt.bodySize > limit {
				wantOutput = fmt.Sprintf("fetch error: reading %s/metrics: response exceeds limit of %d bytes\n", server.URL, limit)
			}
			if string(output) != wantOutput {
				t.Errorf("output = %q, want %q", output, wantOutput)
			}
			if wantRead := min(tt.bodySize, limit+1); counter.body.bytesRead != wantRead {
				t.Errorf("read %d bytes, want %d", counter.body.bytesRead, wantRead)
			}
			if !counter.body.closed {
				t.Error("response body was not closed")
			}
		})
	}
}
