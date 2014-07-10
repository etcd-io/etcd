/*
Copyright 2013 CoreOS Inc.

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

package http

import (
	"fmt"
	"net/http"
	"net/url"
)

type CORSInfo map[string]bool

func NewCORSInfo(origins []string) (*CORSInfo, error) {
	// Construct a lookup of all origins.
	m := make(map[string]bool)
	for _, v := range origins {
		if v != "*" {
			if _, err := url.Parse(v); err != nil {
				return nil, fmt.Errorf("Invalid CORS origin: %s", err)
			}
		}
		m[v] = true
	}

	info := CORSInfo(m)
	return &info, nil
}

// OriginAllowed determines whether the server will allow a given CORS origin.
func (c CORSInfo) OriginAllowed(origin string) bool {
	return c["*"] || c[origin]
}

type CORSHandler struct {
	Handler http.Handler
	Info    *CORSInfo
}

// addHeader adds the correct cors headers given an origin
func (h *CORSHandler) addHeader(w http.ResponseWriter, origin string) {
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Headers", "accept, content-type")
}

// ServeHTTP adds the correct CORS headers based on the origin and returns immediately
// with a 200 OK if the method is OPTIONS.
func (h *CORSHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// It is important to flush before leaving the goroutine.
	// Or it may miss the latest info written.
	defer w.(http.Flusher).Flush()

	// Write CORS header.
	if h.Info.OriginAllowed("*") {
		h.addHeader(w, "*")
	} else if origin := req.Header.Get("Origin"); h.Info.OriginAllowed(origin) {
		h.addHeader(w, origin)
	}

	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.Handler.ServeHTTP(w, req)
}
