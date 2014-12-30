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

package testutil

import (
	"net/url"
	"runtime"
	"testing"

	"github.com/coreos/etcd/pkg/types"
)

// WARNING: This is a hack.
// Remove this when we are able to block/check the status of the go-routines.
func ForceGosched() {
	// possibility enough to sched up to 10 go routines.
	for i := 0; i < 10000; i++ {
		runtime.Gosched()
	}
}

func MustNewURLs(t *testing.T, urls []string) []url.URL {
	if urls == nil {
		return nil
	}
	u, err := types.NewURLs(urls)
	if err != nil {
		t.Fatalf("unexpected new urls error: %v", err)
	}
	return u
}
