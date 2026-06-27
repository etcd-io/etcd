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
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	ErrNoExist  = fmt.Errorf("failpoint: failpoint does not exist")
	ErrDisabled = fmt.Errorf("failpoint: failpoint is disabled")

	failpoints map[string]*Failpoint
	// failpointsMu protects the failpoints map, preventing concurrent
	// accesses during commands such as Enabling and Disabling
	failpointsMu sync.RWMutex

	envTerms map[string]string

	// panicMu (panic mutex) ensures that the action of panic failpoints
	// and serving of the HTTP requests won't be executed at the same time,
	// avoiding the possibility that the server runtime panics during processing
	// requests
	panicMu sync.Mutex
)

func init() {
	failpoints = make(map[string]*Failpoint)
	envTerms = make(map[string]string)
	if s := os.Getenv("GOFAIL_FAILPOINTS"); len(s) > 0 {
		fpMap, err := parseFailpoints(s)
		if err != nil {
			fmt.Printf("fail to parse failpoint: %v\n", err)
			os.Exit(1)
		}
		envTerms = fpMap
	}
	if s := os.Getenv("GOFAIL_HTTP"); len(s) > 0 {
		if err := serve(s); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

func parseFailpoints(fps string) (map[string]string, error) {
	// The format is <FAILPOINT>=<TERMS>[;<FAILPOINT>=<TERMS>]*
	fpMap := map[string]string{}

	for _, fp := range strings.Split(fps, ";") {
		if len(fp) == 0 {
			continue
		}
		fpTerm := strings.Split(fp, "=")
		if len(fpTerm) != 2 {
			err := fmt.Errorf("bad failpoint %q", fp)
			return nil, err
		}
		fpMap[fpTerm[0]] = fpTerm[1]
	}
	return fpMap, nil
}

// Enable sets a failpoint to a given failpoint description.
func Enable(name, inTerms string) error {
	failpointsMu.RLock()
	fp := failpoints[name]
	failpointsMu.RUnlock()
	if fp == nil {
		return ErrNoExist
	}

	t, err := newTerms(name, inTerms)
	if err != nil {
		fmt.Printf("failed to enable \"%s=%s\" (%v)\n", name, inTerms, err)
		return err
	}

	fp.SetTerm(t)

	return nil
}

// Disable stops a failpoint from firing.
func Disable(name string) error {
	failpointsMu.RLock()
	fp := failpoints[name]
	failpointsMu.RUnlock()
	if fp == nil {
		return ErrNoExist
	}

	return fp.ClearTerm()
}

// Status gives the current setting and execution count for the failpoint
func Status(failpath string) (string, int, error) {
	failpointsMu.RLock()
	fp := failpoints[failpath]
	failpointsMu.RUnlock()
	if fp == nil {
		return "", 0, ErrNoExist
	}

	return fp.Status()
}

func List() []string {
	failpointsMu.Lock()
	defer failpointsMu.Unlock()
	return list()
}

func list() []string {
	ret := make([]string, 0, len(failpoints))
	for fp := range failpoints {
		ret = append(ret, fp)
	}
	return ret
}

func register(name string) *Failpoint {
	failpointsMu.Lock()
	if _, ok := failpoints[name]; ok {
		failpointsMu.Unlock()
		panic(fmt.Sprintf("failpoint name %s is already registered.", name))
	}

	fp := &Failpoint{}
	failpoints[name] = fp
	failpointsMu.Unlock()
	if t, ok := envTerms[name]; ok {
		Enable(name, t)
	}
	return fp
}
