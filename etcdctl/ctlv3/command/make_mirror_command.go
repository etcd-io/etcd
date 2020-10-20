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

package command

import (
	"context"
	"errors"
	"fmt"
	"github.com/bgentry/speakeasy"
	"strings"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/mirror"

	"github.com/spf13/cobra"
)

var (
	mminsecureTr   bool
	mmcert         string
	mmkey          string
	mmcacert       string
	mmprefix       string
	mmdestprefix   string
	mmuser         string
	mmpassword     string
	mmnodestprefix bool
)

// NewMakeMirrorCommand returns the cobra command for "makeMirror".
func NewMakeMirrorCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "make-mirror [options] <destination>",
		Short: "Makes a mirror at the destination etcd cluster",
		Run:   makeMirrorCommandFunc,
	}

	c.Flags().StringVar(&mmprefix, "prefix", "", "Key-value prefix to mirror")
	c.Flags().StringVar(&mmdestprefix, "dest-prefix", "", "destination prefix to mirror a prefix to a different prefix in the destination cluster")
	c.Flags().BoolVar(&mmnodestprefix, "no-dest-prefix", false, "mirror key-values to the root of the destination cluster")
	c.Flags().StringVar(&mmcert, "dest-cert", "", "Identify secure client using this TLS certificate file for the destination cluster")
	c.Flags().StringVar(&mmkey, "dest-key", "", "Identify secure client using this TLS key file")
	c.Flags().StringVar(&mmcacert, "dest-cacert", "", "Verify certificates of TLS enabled secure servers using this CA bundle")
	// TODO: secure by default when etcd enables secure gRPC by default.
	c.Flags().BoolVar(&mminsecureTr, "dest-insecure-transport", true, "Disable transport security for client connections")
	c.Flags().StringVar(&mmuser, "dest-user", "", "Destination username[:password] for authentication (prompt if password is not supplied)")
	c.Flags().StringVar(&mmpassword, "dest-password", "", "Destination password for authentication (if this option is used, --user option shouldn't include password)")

	return c
}

func authDestCfg() *authCfg {
	if mmuser == "" {
		return nil
	}

	var cfg authCfg

	if mmpassword == "" {
		splitted := strings.SplitN(mmuser, ":", 2)
		if len(splitted) < 2 {
			var err error
			cfg.username = mmuser
			cfg.password, err = speakeasy.Ask("Destination Password: ")
			if err != nil {
				ExitWithError(ExitError, err)
			}
		} else {
			cfg.username = splitted[0]
			cfg.password = splitted[1]
		}
	} else {
		cfg.username = mmuser
		cfg.password = mmpassword
	}

	return &cfg
}

func makeMirrorCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ExitWithError(ExitBadArgs, errors.New("make-mirror takes one destination argument"))
	}

	dialTimeout := dialTimeoutFromCmd(cmd)
	keepAliveTime := keepAliveTimeFromCmd(cmd)
	keepAliveTimeout := keepAliveTimeoutFromCmd(cmd)
	sec := &secureCfg{
		cert:              mmcert,
		key:               mmkey,
		cacert:            mmcacert,
		insecureTransport: mminsecureTr,
	}

	auth := authDestCfg()

	cc := &clientConfig{
		endpoints:        []string{args[0]},
		dialTimeout:      dialTimeout,
		keepAliveTime:    keepAliveTime,
		keepAliveTimeout: keepAliveTimeout,
		scfg:             sec,
		acfg:             auth,
	}
	dc := cc.mustClient()
	c := mustClientFromCmd(cmd)

	err := makeMirror(context.TODO(), c, dc)
	ExitWithError(ExitError, err)
}

func makeMirror(ctx context.Context, c *clientv3.Client, dc *clientv3.Client) error {
	total := int64(0)

	go func() {
		for {
			time.Sleep(30 * time.Second)
			fmt.Println(atomic.LoadInt64(&total))
		}
	}()

	s := mirror.NewSyncer(c, mmprefix, 0)

	rc, errc := s.SyncBase(ctx)

	// if destination prefix is specified and remove destination prefix is true return error
	if mmnodestprefix && len(mmdestprefix) > 0 {
		ExitWithError(ExitBadArgs, fmt.Errorf("`--dest-prefix` and `--no-dest-prefix` cannot be set at the same time, choose one"))
	}

	// if remove destination prefix is false and destination prefix is empty set the value of destination prefix same as prefix
	if !mmnodestprefix && len(mmdestprefix) == 0 {
		mmdestprefix = mmprefix
	}

	for r := range rc {
		for _, kv := range r.Kvs {
			_, err := dc.Put(ctx, modifyPrefix(string(kv.Key)), string(kv.Value))
			if err != nil {
				return err
			}
			atomic.AddInt64(&total, 1)
		}
	}

	err := <-errc
	if err != nil {
		return err
	}

	wc := s.SyncUpdates(ctx)

	for wr := range wc {
		if wr.CompactRevision != 0 {
			return rpctypes.ErrCompacted
		}

		var lastRev int64
		ops := []clientv3.Op{}

		for _, ev := range wr.Events {
			nextRev := ev.Kv.ModRevision
			if lastRev != 0 && nextRev > lastRev {
				_, err := dc.Txn(ctx).Then(ops...).Commit()
				if err != nil {
					return err
				}
				ops = []clientv3.Op{}
			}
			lastRev = nextRev
			switch ev.Type {
			case mvccpb.PUT:
				ops = append(ops, clientv3.OpPut(modifyPrefix(string(ev.Kv.Key)), string(ev.Kv.Value)))
				atomic.AddInt64(&total, 1)
			case mvccpb.DELETE:
				ops = append(ops, clientv3.OpDelete(modifyPrefix(string(ev.Kv.Key))))
				atomic.AddInt64(&total, 1)
			default:
				panic("unexpected event type")
			}
		}

		if len(ops) != 0 {
			_, err := dc.Txn(ctx).Then(ops...).Commit()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func modifyPrefix(key string) string {
	return strings.Replace(key, mmprefix, mmdestprefix, 1)
}
