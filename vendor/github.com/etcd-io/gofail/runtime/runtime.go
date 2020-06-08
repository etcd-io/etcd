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

package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	ErrNoExist  = fmt.Errorf("failpoint: failpoint does not exist")
	ErrDisabled = fmt.Errorf("failpoint: failpoint is disabled")

	failpoints   map[string]*Failpoint
	failpointsMu sync.RWMutex
	envTerms     map[string]string
)

func init() {
	failpoints = make(map[string]*Failpoint)
	envTerms = make(map[string]string)
	if s := os.Getenv("GOFAIL_FAILPOINTS"); len(s) > 0 {
		// format is <FAILPOINT>=<TERMS>[;<FAILPOINT>=<TERMS>;...]
		for _, fp := range strings.Split(s, ";") {
			fp_term := strings.Split(fp, "=")
			if len(fp_term) != 2 {
				fmt.Printf("bad failpoint %q\n", fp)
				os.Exit(1)
			}
			envTerms[fp_term[0]] = fp_term[1]
		}
	}
	if s := os.Getenv("GOFAIL_HTTP"); len(s) > 0 {
		if err := serve(s); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

// Enable sets a failpoint to a given failpoint description.
func Enable(failpath, inTerms string) error {
	unlock, err := enableAndLock(failpath, inTerms)
	if unlock != nil {
		unlock()
	}
	return err
}

// enableAndLock enables a failpoint and returns a function to unlock it
func enableAndLock(failpath, inTerms string) (func(), error) {
	t, err := newTerms(failpath, inTerms)
	if err != nil {
		fmt.Printf("failed to enable \"%s=%s\" (%v)\n", failpath, inTerms, err)
		return nil, err
	}
	failpointsMu.RLock()
	fp := failpoints[failpath]
	failpointsMu.RUnlock()
	if fp == nil {
		return nil, ErrNoExist
	}
	fp.mu.Lock()
	fp.t = t
	fp.released = false
	return func() { fp.mu.Unlock() }, nil
}

// Disable stops a failpoint from firing.
func Disable(failpath string) error {
	failpointsMu.RLock()
	fp := failpoints[failpath]
	failpointsMu.RUnlock()
	if fp == nil {
		return ErrNoExist
	}

	fp.cmu.RLock()
	cancel := fp.cancel
	donec := fp.donec
	fp.cmu.RUnlock()
	if cancel != nil && donec != nil {
		cancel()
		<-donec

		fp.cmu.Lock()
		fp.ctx, fp.cancel = context.WithCancel(context.Background())
		fp.donec = make(chan struct{})
		fp.released = true
		fp.cmu.Unlock()
	}

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if fp.t == nil {
		return ErrDisabled
	}
	fp.t = nil
	return nil
}

// Status gives the current setting for the failpoint
func Status(failpath string) (string, error) {
	failpointsMu.RLock()
	fp := failpoints[failpath]
	failpointsMu.RUnlock()
	if fp == nil {
		return "", ErrNoExist
	}
	fp.mu.RLock()
	t := fp.t
	fp.mu.RUnlock()
	if t == nil {
		return "", ErrDisabled
	}
	return t.desc, nil
}

func List() []string {
	failpointsMu.RLock()
	ret := make([]string, 0, len(failpoints))
	for fp := range failpoints {
		ret = append(ret, fp)
	}
	failpointsMu.RUnlock()
	return ret
}

func register(failpath string, fp *Failpoint) {
	failpointsMu.Lock()
	failpoints[failpath] = fp
	failpointsMu.Unlock()
	if t, ok := envTerms[failpath]; ok {
		Enable(failpath, t)
	}
}
