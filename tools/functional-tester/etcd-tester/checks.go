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
	"reflect"
	"time"

	"golang.org/x/net/context"
)

const (
	retries             = 7
	stabilizationPeriod = 3 * time.Second
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

const leaseCheckerTimeout = 10 * time.Second

func (hc *hashChecker) checkRevAndHashes() (err error) {
	// retries in case of transient failure or etcd nodes have not stablized yet.
	var (
		revsStable   bool
		hashesStable bool
	)
	for i := 0; i < retries; i++ {
		revsStable, err = hc.areRevisonsStable()
		if err != nil || !revsStable {
			continue
		}
		hashesStable, err = hc.areHashesStable()
		if err != nil || !hashesStable {
			continue
		}
		// hashes must be stable at this point
		return nil
	}

	if err != nil {
		return err
	}

	if !revsStable || !hashesStable {
		return fmt.Errorf("checkRevAndHashes detects inconsistency: [revisions stable %v] [hashes stable %v]", revsStable, hashesStable)
	}

	return err
}

func (hc *hashChecker) areRevisonsStable() (rv bool, err error) {
	var preRevs map[string]int64
	for i := 0; i < 2; i++ {
		revs, _, err := hc.hrg.getRevisionHash()
		if err != nil {
			return false, err
		}

		_, sameRev := getSameValue(revs)
		if !sameRev {
			plog.Printf("current revisions are not consistent: revisions [revisions: %v]", revs)
			return false, nil
		}
		// sleep for N seconds. after that, check to make sure that revisions don't change
		if i == 0 {
			preRevs = revs
			time.Sleep(stabilizationPeriod)
		} else if !reflect.DeepEqual(revs, preRevs) {
			// use map comparison logic found in http://stackoverflow.com/questions/18208394/testing-equivalence-of-maps-golang
			plog.Printf("revisions are not stable: [current revisions: %v] [previous revisions: %v]", revs, preRevs)
			return false, nil
		}
	}
	plog.Printf("revisions are stable: revisions [revisions: %v]", preRevs)
	return true, nil
}

func (hc *hashChecker) areHashesStable() (rv bool, err error) {
	var prevHashes map[string]int64
	for i := 0; i < 2; i++ {
		revs, hashes, err := hc.hrg.getRevisionHash()
		if err != nil {
			return false, err
		}
		_, sameRev := getSameValue(revs)
		_, sameHashes := getSameValue(hashes)
		if !sameRev || !sameHashes {
			plog.Printf("hashes are not stable: revisions [revisions: %v] and hashes [hashes: %v]", revs, hashes)
			return false, nil
		}
		// sleep for N seconds. after that, check to make sure that the hashes and revisions don't change
		if i == 0 {
			time.Sleep(stabilizationPeriod)
			prevHashes = hashes
		} else if !reflect.DeepEqual(hashes, prevHashes) {
			// use map comparison logic found in http://stackoverflow.com/questions/18208394/testing-equivalence-of-maps-golang
			plog.Printf("hashes are not stable: [current hashes: %v] [previous hashes: %v]", hashes, prevHashes)
			return false, nil
		}
	}
	plog.Printf("hashes are stable: hashes [hashes: %v]", prevHashes)
	return true, nil
}

func (hc *hashChecker) Check() error {
	return hc.checkRevAndHashes()
}

type leaseChecker struct{ ls *leaseStresser }

func (lc *leaseChecker) Check() error {
	plog.Infof("checking revoked leases %v", lc.ls.revokedLeases.leases)
	if err := lc.check(true, lc.ls.revokedLeases.leases); err != nil {
		return err
	}
	plog.Infof("checking alive leases %v", lc.ls.aliveLeases.leases)
	if err := lc.check(false, lc.ls.aliveLeases.leases); err != nil {
		return err
	}
	plog.Infof("checking short lived leases %v", lc.ls.shortLivedLeases.leases)
	return lc.check(true, lc.ls.shortLivedLeases.leases)
}

func (lc *leaseChecker) check(expired bool, leases map[int64]time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), leaseCheckerTimeout)
	defer cancel()
	for leaseID := range leases {
		keysExpired, err := lc.ls.hasKeysAttachedToLeaseExpired(ctx, leaseID)
		if err != nil {
			plog.Errorf("hasKeysAttachedToLeaseExpired error: (%v)", err)
			return err
		}
		leaseExpired, err := lc.ls.hasLeaseExpired(ctx, leaseID)
		if err != nil {
			plog.Errorf("hasLeaseExpired error: (%v)", err)
			return err
		}
		if leaseExpired != keysExpired {
			return fmt.Errorf("lease %v expiration mismatch (lease expired=%v, keys expired=%v)", leaseID, leaseExpired, keysExpired)
		}
		if leaseExpired != expired {
			return fmt.Errorf("lease %v expected expired=%v, got %v", leaseID, expired, leaseExpired)
		}
	}
	return nil
}

// compositeChecker implements a checker that runs a slice of Checkers concurrently.
type compositeChecker struct{ checkers []Checker }

func newCompositeChecker(checkers []Checker) Checker {
	return &compositeChecker{checkers}
}

func (cchecker *compositeChecker) Check() error {
	errc := make(chan error)
	for _, c := range cchecker.checkers {
		go func(chk Checker) { errc <- chk.Check() }(c)
	}
	var errs []error
	for range cchecker.checkers {
		if err := <-errc; err != nil {
			errs = append(errs, err)
		}
	}
	return errsToError(errs)
}

type noChecker struct{}

func newNoChecker() Checker        { return &noChecker{} }
func (nc *noChecker) Check() error { return nil }
