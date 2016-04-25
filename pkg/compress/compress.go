// Copyright 2016 CoreOS, Inc.
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

package compress

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/golang/snappy"
)

type Type int

const (
	NoCompress Type = iota
	Gzip
	Snappy
)

var (
	headers = [...]string{
		"",
		"gzip",
		"snappy",
	}

	// re-use between goroutines
	gzipWriterPool sync.Pool
	gzipReaderPool sync.Pool

	snappyWriterPool sync.Pool
	snappyReaderPool sync.Pool
)

func (ct Type) String() string {
	return headers[ct]
}

func ParseType(opt string) Type {
	switch strings.ToLower(opt) {
	case "gzip":
		return Gzip
	case "snappy":
		return Snappy
	default:
		return NoCompress
	}
}

func NewRequest(req *http.Request, comp Type) *http.Request {
	req.Header.Set("Accept-Encoding", headers[comp])
	return req
}

type ResponseWriter struct {
	rw     http.ResponseWriter
	writer io.Writer

	gzipWriter   *gzip.Writer
	snappyWriter *snappy.Writer
}

// NewResponseWriter returns http.ResponseWriter wrapper with compressions.
// If 'Accept-Encoding' header is not specified, it defaults to regular http.ResponseWriter.
func NewResponseWriter(rw http.ResponseWriter, req *http.Request) *ResponseWriter {
	rw.Header().Set("Vary", "Accept-Encoding")
	rw.Header().Set("Cache-Control", "no-cache") // disable response caching

	crw := &ResponseWriter{}
	crw.rw = rw

	switch req.Header.Get("Accept-Encoding") {
	case "gzip":
		rw.Header().Set("Content-Encoding", "gzip")
		if wp := gzipWriterPool.Get(); wp != nil {
			gzipWriter := wp.(*gzip.Writer)
			gzipWriter.Reset(rw)
			crw.gzipWriter = gzipWriter
		} else {
			crw.gzipWriter = gzip.NewWriter(rw)
		}
		crw.writer = crw.gzipWriter

	case "snappy":
		rw.Header().Set("Content-Encoding", "snappy")
		if wp := snappyWriterPool.Get(); wp != nil {
			snappyWriter := wp.(*snappy.Writer)
			snappyWriter.Reset(rw)
			crw.snappyWriter = snappyWriter
		} else {
			crw.snappyWriter = snappy.NewBufferedWriter(rw)
		}
		crw.writer = crw.snappyWriter

	default:
		// default to plain-text
		crw.writer = rw
	}

	return crw
}

// Header satisfies http.ResponseWriter interface.
func (crw *ResponseWriter) Header() http.Header {
	return crw.rw.Header()
}

// Write satisfies io.Writer and http.ResponseWriter interfaces.
func (crw *ResponseWriter) Write(b []byte) (int, error) {
	return crw.writer.Write(b)
}

// WriteHeader satisfies http.ResponseWriter interface.
func (crw *ResponseWriter) WriteHeader(status int) {
	crw.rw.WriteHeader(status)
}

// Close closes ResponseWriter putting writers back into pool.
func (crw *ResponseWriter) Close() {
	switch {
	case crw.gzipWriter != nil:
		crw.gzipWriter.Close()
		gzipWriterPool.Put(crw.gzipWriter)
		crw.gzipWriter = nil

	case crw.snappyWriter != nil:
		crw.snappyWriter.Close()
		snappyWriterPool.Put(crw.snappyWriter)
		crw.snappyWriter = nil

	default:
		crw.writer = nil
	}
}

// NewHandler wraps a http.Handler to support compressions.
func NewHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		respWriter := NewResponseWriter(rw, req)
		defer respWriter.Close()

		h.ServeHTTP(respWriter, req)
	})
}

type ResponseReader struct {
	resp   *http.Response
	reader io.Reader

	gzipReader   *gzip.Reader
	snappyReader *snappy.Reader
}

// NewResponseReader returns http.Response wrapper with compression reader.
func NewResponseReader(resp *http.Response) *ResponseReader {
	crd := &ResponseReader{}
	crd.resp = resp

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		if rp := gzipReaderPool.Get(); rp != nil {
			gzipReader := rp.(*gzip.Reader)
			gzipReader.Reset(resp.Body)
			crd.gzipReader = gzipReader
		} else {
			crd.gzipReader, _ = gzip.NewReader(resp.Body)
		}
		crd.reader = crd.gzipReader

	case "snappy":
		if rp := snappyReaderPool.Get(); rp != nil {
			snappyReader := rp.(*snappy.Reader)
			snappyReader.Reset(resp.Body)
			crd.snappyReader = snappyReader
		} else {
			crd.snappyReader = snappy.NewReader(resp.Body)
		}
		crd.reader = crd.snappyReader

	default:
		// default to plain-text
		crd.reader = resp.Body
	}

	return crd
}

func (crd *ResponseReader) Read(p []byte) (int, error) {
	return crd.reader.Read(p)
}

func (crd *ResponseReader) Close() {
	switch {
	case crd.gzipReader != nil:
		crd.gzipReader.Close()
		gzipReaderPool.Put(crd.gzipReader)
		crd.gzipReader = nil

	case crd.snappyReader != nil:
		snappyReaderPool.Put(crd.snappyReader)
		crd.snappyReader = nil

	default:
		crd.reader = nil
	}

	// drains http.Response.Body until it hits EOF
	// and closes it. This prevents TCP/TLS connections from closing,
	// therefore available for reuse.
	io.Copy(ioutil.Discard, crd.resp.Body)
	crd.resp.Body.Close()
}
