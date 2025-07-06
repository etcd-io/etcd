// Copyright 2020 The etcd Authors
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

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type value struct {
	val     string
	version int64
}

var data = make(map[string]*value)

type request struct {
	Op      string `json:"op"`
	Key     string `json:"key"`
	Val     string `json:"val"`
	Version int64  `json:"version"`
}

type response struct {
	Val     string `json:"val"`
	Version int64  `json:"version"`
	Err     string `json:"err"`
}

func writeResponse(resp response, w http.ResponseWriter) {
	wBytes, err := json.Marshal(resp)
	if err != nil {
		fmt.Printf("failed to marshal json: %s\n", err)
		os.Exit(1)
	}
	_, err = w.Write(wBytes)
	if err != nil {
		fmt.Printf("failed to write a response: %s\n", err)
		os.Exit(1)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	rBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("failed to read http request: %s\n", err)
		os.Exit(1)
	}

	var req request
	err = json.Unmarshal(rBytes, &req)
	if err != nil {
		fmt.Printf("failed to unmarshal json: %s\n", err)
		os.Exit(1)
	}

	if strings.Compare(req.Op, "read") == 0 {
		if val, ok := data[req.Key]; ok {
			writeResponse(response{val.val, val.version, ""}, w)
		} else {
			writeResponse(response{"", -1, "key not found"}, w)
		}
	} else if strings.Compare(req.Op, "write") == 0 {
		if val, ok := data[req.Key]; ok {
			if req.Version != val.version {
				writeResponse(response{"", -1, fmt.Sprintf("given version (%d) is different from the existing version (%d)", req.Version, val.version)}, w)
			} else {
				data[req.Key].val = req.Val
				data[req.Key].version = req.Version
				writeResponse(response{req.Val, req.Version, ""}, w)
			}
		} else {
			data[req.Key] = &value{req.Val, req.Version}
			writeResponse(response{req.Val, req.Version, ""}, w)
		}
	} else {
		fmt.Printf("unknown op: %s\n", req.Op)
		return
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
