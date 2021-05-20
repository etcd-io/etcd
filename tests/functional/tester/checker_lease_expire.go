// Copyright 2018 The etcd Authors
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

package tester

import (
	"context"
	"fmt"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/functional/rpcpb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type leaseExpireChecker struct {
	ctype rpcpb.Checker
	lg    *zap.Logger
	m     *rpcpb.Member
	ls    *leaseStresser
	cli   *clientv3.Client
}

func newLeaseExpireChecker(ls *leaseStresser) Checker {
	return &leaseExpireChecker{
		ctype: rpcpb.Checker_LEASE_EXPIRE,
		lg:    ls.lg,
		m:     ls.m,
		ls:    ls,
	}
}

func (lc *leaseExpireChecker) Type() rpcpb.Checker {
	return lc.ctype
}

func (lc *leaseExpireChecker) EtcdClientEndpoints() []string {
	return []string{lc.m.EtcdClientEndpoint}
}

func (lc *leaseExpireChecker) Check() error {
	if lc.ls == nil {
		return nil
	}
	if lc.ls != nil &&
		(lc.ls.revokedLeases == nil ||
			lc.ls.aliveLeases == nil ||
			lc.ls.shortLivedLeases == nil) {
		return nil
	}

	cli, err := lc.m.CreateEtcdClient(grpc.WithBackoffMaxDelay(time.Second))
	if err != nil {
		return fmt.Errorf("%v (%q)", err, lc.m.EtcdClientEndpoint)
	}
	defer func() {
		if cli != nil {
			cli.Close()
		}
	}()
	lc.cli = cli

	if err := check(lc.lg, lc.cli, true, lc.ls.revokedLeases.leases); err != nil {
		return err
	}
	if err := check(lc.lg, lc.cli, false, lc.ls.aliveLeases.leases); err != nil {
		return err
	}

	return lc.checkShortLivedLeases()
}

const leaseExpireCheckerTimeout = 10 * time.Second

// checkShortLivedLeases ensures leases expire.
func (lc *leaseExpireChecker) checkShortLivedLeases() error {
	ctx, cancel := context.WithTimeout(context.Background(), leaseExpireCheckerTimeout)
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

func (lc *leaseExpireChecker) checkShortLivedLease(ctx context.Context, leaseID int64) (err error) {
	// retry in case of transient failure or lease is expired but not yet revoked due to the fact that etcd cluster didn't have enought time to delete it.
	var resp *clientv3.LeaseTimeToLiveResponse
	for i := 0; i < retries; i++ {
		resp, err = getLeaseByID(ctx, lc.cli, leaseID)
		// lease not found, for ~v3.1 compatibilities, check ErrLeaseNotFound
		if (err == nil && resp.TTL == -1) || (err != nil && rpctypes.Error(err) == rpctypes.ErrLeaseNotFound) {
			return nil
		}
		if err != nil {
			lc.lg.Debug(
				"retrying; Lease TimeToLive failed",
				zap.Int("retries", i),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Error(err),
			)
			continue
		}
		if resp.TTL > 0 {
			dur := time.Duration(resp.TTL) * time.Second
			lc.lg.Debug(
				"lease has not been expired, wait until expire",
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Int64("ttl", resp.TTL),
				zap.Duration("wait-duration", dur),
			)
			time.Sleep(dur)
		} else {
			lc.lg.Debug(
				"lease expired but not yet revoked",
				zap.Int("retries", i),
				zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
				zap.Int64("ttl", resp.TTL),
				zap.Duration("wait-duration", time.Second),
			)
			time.Sleep(time.Second)
		}
		if err = checkLease(ctx, lc.lg, lc.cli, false, leaseID); err != nil {
			continue
		}
		return nil
	}
	return err
}

func checkLease(ctx context.Context, lg *zap.Logger, cli *clientv3.Client, expired bool, leaseID int64) error {
	keysExpired, err := hasKeysAttachedToLeaseExpired(ctx, lg, cli, leaseID)
	if err != nil {
		lg.Warn(
			"hasKeysAttachedToLeaseExpired failed",
			zap.Any("endpoint", cli.Endpoints()),
			zap.Error(err),
		)
		return err
	}
	leaseExpired, err := hasLeaseExpired(ctx, lg, cli, leaseID)
	if err != nil {
		lg.Warn(
			"hasLeaseExpired failed",
			zap.Any("endpoint", cli.Endpoints()),
			zap.Error(err),
		)
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

func check(lg *zap.Logger, cli *clientv3.Client, expired bool, leases map[int64]time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), leaseExpireCheckerTimeout)
	defer cancel()
	for leaseID := range leases {
		if err := checkLease(ctx, lg, cli, expired, leaseID); err != nil {
			return err
		}
	}
	return nil
}

// TODO: handle failures from "grpc.WaitForReady(true)"
func getLeaseByID(ctx context.Context, cli *clientv3.Client, leaseID int64) (*clientv3.LeaseTimeToLiveResponse, error) {
	return cli.TimeToLive(
		ctx,
		clientv3.LeaseID(leaseID),
		clientv3.WithAttachedKeys(),
	)
}

func hasLeaseExpired(ctx context.Context, lg *zap.Logger, cli *clientv3.Client, leaseID int64) (bool, error) {
	// keep retrying until lease's state is known or ctx is being canceled
	for ctx.Err() == nil {
		resp, err := getLeaseByID(ctx, cli, leaseID)
		if err != nil {
			// for ~v3.1 compatibilities
			if rpctypes.Error(err) == rpctypes.ErrLeaseNotFound {
				return true, nil
			}
		} else {
			return resp.TTL == -1, nil
		}
		lg.Warn(
			"hasLeaseExpired getLeaseByID failed",
			zap.Any("endpoint", cli.Endpoints()),
			zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
			zap.Error(err),
		)
	}
	return false, ctx.Err()
}

// The keys attached to the lease has the format of "<leaseID>_<idx>" where idx is the ordering key creation
// Since the format of keys contains about leaseID, finding keys base on "<leaseID>" prefix
// determines whether the attached keys for a given leaseID has been deleted or not
func hasKeysAttachedToLeaseExpired(ctx context.Context, lg *zap.Logger, cli *clientv3.Client, leaseID int64) (bool, error) {
	resp, err := cli.Get(ctx, fmt.Sprintf("%d", leaseID), clientv3.WithPrefix())
	if err != nil {
		lg.Warn(
			"hasKeysAttachedToLeaseExpired failed",
			zap.Any("endpoint", cli.Endpoints()),
			zap.String("lease-id", fmt.Sprintf("%016x", leaseID)),
			zap.Error(err),
		)
		return false, err
	}
	return len(resp.Kvs) == 0, nil
}
