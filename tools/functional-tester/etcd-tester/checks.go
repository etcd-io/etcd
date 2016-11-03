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

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
)

const (
	retries = 7
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
	var (
		revs   map[string]int64
		hashes map[string]int64
	)

	// retries in case of transient failure or etcd cluster has not stablized yet.
	for i := 0; i < retries; i++ {
		revs, hashes, err = hc.hrg.getRevisionHash()
		if err != nil {
			plog.Warningf("retry %d. failed to retrieve revison and hash (%v)", i, err)
		} else {
			sameRev := getSameValue(revs)
			sameHashes := getSameValue(hashes)
			if sameRev && sameHashes {
				return nil
			}
			plog.Warningf("retry %d. etcd cluster is not stable: [revisions: %v] and [hashes: %v]", i, revs, hashes)
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
	return lc.checkShortLivedLeases()

}

// checkShortLivedLeases ensures leases expire.
func (lc *leaseChecker) checkShortLivedLeases() error {
	ctx, cancel := context.WithTimeout(context.Background(), leaseCheckerTimeout)
	errc := make(chan error)
	defer cancel()
	for leaseID := range lc.ls.shortLivedLeases.leases {
		go func(id int64) {
			errc <- lc.checkShortLivedLease(ctx, id)
		}(leaseID)
	}

	var errs []error
	for range lc.ls.shortLivedLeases.leases {
		if err := <-errc; err != nil {
			errs = append(errs, err)
		}
	}
	return errsToError(errs)
}

func (lc *leaseChecker) checkShortLivedLease(ctx context.Context, leaseID int64) (err error) {
	// retry in case of transient failure or lease is expired but not yet revoked due to the fact that etcd cluster didn't have enought time to delete it.
	var resp *pb.LeaseTimeToLiveResponse
	for i := 0; i < retries; i++ {
		resp, err = lc.ls.getLeaseByID(ctx, leaseID)
		if rpctypes.Error(err) == rpctypes.ErrLeaseNotFound {
			return nil
		}
		if err != nil {
			plog.Warningf("retry %d. failed to retrieve lease %v error (%v)", i, leaseID, err)
			continue
		}
		if resp.TTL > 0 {
			plog.Warningf("lease %v is not expired. sleep for %d until it expires.", leaseID, resp.TTL)
			time.Sleep(time.Duration(resp.TTL) * time.Second)
		} else {
			plog.Warningf("retry %d. lease %v is expired but not yet revoked", i, leaseID)
			time.Sleep(time.Second)
		}
		if err = lc.checkLease(ctx, false, leaseID); err != nil {
			continue
		}
		return nil
	}
	return err
}

func (lc *leaseChecker) checkLease(ctx context.Context, expired bool, leaseID int64) error {
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
	return nil
}

func (lc *leaseChecker) check(expired bool, leases map[int64]time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), leaseCheckerTimeout)
	defer cancel()
	for leaseID := range leases {
		if err := lc.checkLease(ctx, expired, leaseID); err != nil {
			return err
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
