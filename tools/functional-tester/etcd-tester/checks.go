// Copyright 2016 The etcd Authors
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
	"fmt"
	"time"
)

type Checker interface {
	// Check returns an error if the system fails a consistency check.
	Check() error
}

type hashChecker struct {
	tt *tester
}

func newHashChecker(tt *tester) Checker { return &hashChecker{tt} }

func (hc *hashChecker) Check() (err error) {
	plog.Printf("%s fetching current revisions...", hc.tt.logPrefix())
	var (
		revs   map[string]int64
		hashes map[string]int64
		ok     bool
	)
	for i := 0; i < 7; i++ {
		time.Sleep(time.Second)

		revs, hashes, err = hc.tt.cluster.getRevisionHash()
		if err != nil {
			plog.Printf("%s #%d failed to get current revisions (%v)", hc.tt.logPrefix(), i, err)
			continue
		}
		if _, ok = getSameValue(revs); ok {
			break
		}

		plog.Printf("%s #%d inconsistent current revisions %+v", hc.tt.logPrefix(), i, revs)
	}
	if !ok || err != nil {
		return fmt.Errorf("checking current revisions failed [err: %v, revisions: %v]", err, revs)
	}
	plog.Printf("%s all members are consistent with current revisions [revisions: %v]", hc.tt.logPrefix(), revs)

	plog.Printf("%s checking current storage hashes...", hc.tt.logPrefix())
	if _, ok = getSameValue(hashes); !ok {
		return fmt.Errorf("inconsistent hashes [%v]", hashes)
	}

	plog.Printf("%s all members are consistent with storage hashes", hc.tt.logPrefix())
	return nil
}

type noChecker struct{}

func newNoChecker() Checker        { return &noChecker{} }
func (nc *noChecker) Check() error { return nil }
