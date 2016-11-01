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
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

const (
	hashCheckerRetries = 7

	stabilizeQPS      = 10
	stabilizeDuration = 3 * time.Second

	leaseCheckerTimeout = 10 * time.Second
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

func (hc *hashChecker) checkRevAndHashes() (err error) {
	var (
		revs   map[string]int64
		hashes map[string]int64
	)

	// retries in case of transient failure or etcd cluster has not stablized yet.
	for i := 0; i < hashCheckerRetries; i++ {
		revs, hashes, err = hc.hrg.getRevisionHash()
		if err != nil {
			plog.Warningf("retry %i. failed to retrieve revison and hash (%v)", i, err)
		} else {
			sameRev := getSameValue(revs)
			sameHashes := getSameValue(hashes)
			if sameRev && sameHashes {
				return nil
			}
			plog.Warningf("retry %i. etcd cluster is not stable: [revisions: %v] and [hashes: %v]", i, revs, hashes)
		}
		time.Sleep(time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed revision and hash check (%v)", err)
	}

	return fmt.Errorf("etcd cluster is not stable: [revisions: %v] and [hashes: %v]", revs, hashes)
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

// stabilizeChecker implements a checker that waits for a check to pass,
// then confirms the cluster revision and hash remains the same up to
// some timeout.
type stabilizeChecker struct {
	chk  Checker
	clus *cluster
}

func (sc *stabilizeChecker) Check() error {
	if err := sc.chk.Check(); err != nil {
		return err
	}
	// rev and hash should not change for over stabilization time
	rev, hash, rerr := sc.clus.getRevisionHash()
	if rerr != nil {
		return rerr
	}
	// sample rev and hash until timeout
	ctx, cancel := context.WithTimeout(context.TODO(), stabilizeDuration)
	defer cancel()
	r := rate.NewLimiter(rate.Limit(stabilizeQPS), stabilizeQPS)
	for {
		if err := r.Wait(ctx); err != nil {
			// stable for a while
			break
		}
		newRev, newHash, err := sc.clus.getRevisionHash()
		if err != nil {
			return err
		}
		if reflect.DeepEqual(newRev, rev) && reflect.DeepEqual(newHash, hash) {
			continue
		}
		return fmt.Errorf("stabilize expected (%v,%v), got (%v,%v)", rev, hash, newRev, newHash)
	}
	return nil
}

func NewChecker(s string, c *cluster) Checker {
	types := strings.Split(s, ",")
	if len(types) > 1 {
		chks := make([]Checker, len(types))
		for i, t := range types {
			chks[i] = NewChecker(t, c)
		}
		return &compositeChecker{chks}
	}
	switch s {
	case "nop":
		return newNoChecker()
	case "hash":
		return newHashChecker(c)
	case "stable":
		return &stabilizeChecker{newHashChecker(c), c}
	default:
		panic("unknown checker " + s)
	}
}
