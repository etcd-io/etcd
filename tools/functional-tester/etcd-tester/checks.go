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

type hashAndRevGetter interface {
	getRevisionHash() (revs map[string]int64, hashes map[string]int64, err error)
}

type hashChecker struct {
	hrg hashAndRevGetter
}

func newHashChecker(hrg hashAndRevGetter) Checker { return &hashChecker{hrg} }

func (hc *hashChecker) Check() (err error) {
	plog.Printf("fetching current revisions...")
	var (
		revs   map[string]int64
		hashes map[string]int64
		ok     bool
	)
	for i := 0; i < 7; i++ {
		time.Sleep(time.Second)

		revs, hashes, err = hc.hrg.getRevisionHash()
		if err != nil {
			plog.Printf("#%d failed to get current revisions (%v)", i, err)
			continue
		}
		if _, ok = getSameValue(revs); ok {
			break
		}

		plog.Printf("#%d inconsistent current revisions %+v", i, revs)
	}
	if !ok || err != nil {
		return fmt.Errorf("checking current revisions failed [err: %v, revisions: %v]", err, revs)
	}
	plog.Printf("all members are consistent with current revisions [revisions: %v]", revs)

	plog.Printf("checking current storage hashes...")
	if _, ok = getSameValue(hashes); !ok {
		return fmt.Errorf("inconsistent hashes [%v]", hashes)
	}

	plog.Printf("all members are consistent with storage hashes")
	return nil
}

type noChecker struct{}

func newNoChecker() Checker        { return &noChecker{} }
func (nc *noChecker) Check() error { return nil }
