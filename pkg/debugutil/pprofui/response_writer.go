// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package pprofui

import (
	"io"
	"net/http"
)

// responseBridge is a helper for fetching from the pprof profile handlers.
// Their interface wants a http.ResponseWriter, so we give it one. The writes
// are passed through to an `io.Writer` of our choosing.
type responseBridge struct {
	target     io.Writer
	statusCode int
}

var _ http.ResponseWriter = &responseBridge{}

func (r *responseBridge) Header() http.Header {
	return http.Header{}
}

func (r *responseBridge) Write(b []byte) (int, error) {
	return r.target.Write(b)
}

func (r *responseBridge) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}
