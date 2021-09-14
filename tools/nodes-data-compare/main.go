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
	"context"
	"errors"
	"fmt"
	"github.com/bgentry/speakeasy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc/grpclog"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	rootCommand = &cobra.Command{
		Use:   "etcd-nodes-data-compare",
		Short: "etcd-node-data-compare compare data between etcd nodes.",
	}
	compareNodesDataCommand = &cobra.Command{
		Use:   "compare [endpoints]",
		Short: "compare data between etcd nodes.",
		Run:   compareNodesDataFunc,
	}
)

var (
	endpoints []string
	tls       transport.TLSInfo

	user string

	maxGapRev int64
	limit     int64

	// cache the username and password for multiple connections
	globalUserName string
	globalPassword string

	port string
)

var compareSuccess = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "etcd",
	Subsystem: "tool",
	Name:      "compare_success",
	Help:      "Whether or not node data compare success. 1 is success, 0 is not.",
})

var comparedRev = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: "etcd",
	Subsystem: "tool",
	Name:      "compared_rev",
	Help:      "node compared rev",
})

var lg *zap.Logger

func init() {
	rootCommand.PersistentFlags().StringSliceVar(&endpoints, "endpoints", []string{"127.0.0.1:2379"}, "gRPC endpoints")

	rootCommand.PersistentFlags().Int64Var(&maxGapRev, "maxGapRev", 1000, "Maximum current rev gap between nodes (1000 default)")
	rootCommand.PersistentFlags().Int64Var(&limit, "limit", 500, "Limit keys used for comparing between nodes (500 default)")

	rootCommand.PersistentFlags().StringVar(&tls.CertFile, "cert", "", "identify HTTPS client using this SSL certificate file")
	rootCommand.PersistentFlags().StringVar(&tls.KeyFile, "key", "", "identify HTTPS client using this SSL key file")
	rootCommand.PersistentFlags().StringVar(&tls.TrustedCAFile, "cacert", "", "verify certificates of HTTPS-enabled servers using this CA bundle")

	rootCommand.PersistentFlags().StringVar(&user, "user", "", "provide username[:password] and prompt if password is not supplied.")

	rootCommand.PersistentFlags().StringVar(&port, "port", "", "metrics http port")

	rootCommand.AddCommand(compareNodesDataCommand)

	lg = zap.NewExample()

	prometheus.MustRegister(compareSuccess)
	prometheus.MustRegister(comparedRev)
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
}

func compareNodesDataFunc(cmd *cobra.Command, args []string) {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(port, nil)

	var clients []*clientv3.Client
	for _, endpoint := range endpoints {
		client, err := mustCreateConn(endpoint)
		if err != nil {
			lg.Error("can not create conn", zap.Error(err))
			return
		}
		clients = append(clients, client)
	}

	maxRev, _, err := getMaxLatesRev(clients)
	if err != nil {
		lg.Error("can not get latest rev", zap.Error(err))
		return
	}

	t := time.NewTimer(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		}

		maxRevNew, minRevNew, err := getMaxLatesRev(clients)
		if err != nil {
			lg.Error("can not get latest rev", zap.Error(err))
			return
		}
		lg.Info("compare revision range: ", zap.Int64("maxRev", maxRev), zap.Int64("minRevNew", minRevNew))
		if maxRev > minRevNew {
			lg.Warn("maxRev is smaller than minRevNew", zap.Int64("maxRev", maxRev), zap.Int64("minRevNew", minRevNew))
		}
		err = compare(clients, maxRev, minRevNew)
		comparedRev.Set(float64(minRevNew))
		if err != nil {
			lg.Error("compare failed", zap.Error(err))
			compareSuccess.Set(0)
			return
		}
		maxRev = maxRevNew

		lg.Info("compare success")
		compareSuccess.Set(1)
		t.Reset(30 * time.Second)
	}
}

func getMaxLatesRev(clients []*clientv3.Client) (int64, int64, error) {
	maxRev := int64(0)
	minRev := int64(math.MaxInt64)
	for _, client := range clients {
		rev, err := getLatestRev(client)
		if err != nil {
			return 0, 0, err
		}
		if rev > maxRev {
			maxRev = rev
		}
		if rev < minRev {
			minRev = rev
		}
	}

	if (maxRev - minRev) > maxGapRev {
		lg.Error("current rev gap is too big", zap.Int64("minRev", minRev), zap.Int64("maxRev", maxRev))
		return 0, 0, errors.New("current rev gap is too big")
	}

	return maxRev, minRev, nil
}

func getLatestRev(c *clientv3.Client) (int64, error) {
	resp, err := c.Get(context.Background(), "foo")
	if err != nil {
		return -1, err
	}

	return resp.Header.Revision, nil
}

func compare(clients []*clientv3.Client, oldRev int64, newRev int64) error {
	keyArrays := make([][]*mvccpb.KeyValue, 0)
	for _, client := range clients {
		keyArray, err := getLatestKeys(client, oldRev, newRev)
		if err != nil {
			return err
		}
		keyArrays = append(keyArrays, keyArray)
	}

	return compareLatestKeys(keyArrays)
}

func compareLatestKeys(keyArrays [][]*mvccpb.KeyValue) error {
	if len(keyArrays) == 0 {
		return nil
	}

	keysTotal := len(keyArrays[0])
	for _, keyArray := range keyArrays {
		if len(keyArray) != keysTotal {
			lg.Error("keysTotal compare failee", zap.Int("keysTotal", keysTotal), zap.Int("keyArray-len", len(keyArray)))
			return errors.New("compare keys failed")
		}
	}

	for i := range keyArrays[0] {
		for j := 1; j < len(keyArrays); j++ {
			if strings.Compare(string(keyArrays[0][i].Key), string(keyArrays[j][i].Key)) != 0 || keyArrays[0][i].ModRevision != keyArrays[j][i].ModRevision {
				lg.Error("keys compare failed", zap.String("key-0", string(keyArrays[0][i].Key)), zap.String("key-i", string(keyArrays[j][i].Key)), zap.Int64("key-0-mod", keyArrays[0][i].ModRevision), zap.Int64("key-i-mod", keyArrays[j][i].ModRevision))
				return errors.New("compare keys failed")
			}
		}
	}

	return nil
}

func getLatestKeys(c *clientv3.Client, oldRev int64, newRev int64) ([]*mvccpb.KeyValue, error) {
	opts := []clientv3.OpOption{clientv3.WithLimit(limit), clientv3.WithMinModRev(oldRev), clientv3.WithRev(newRev), clientv3.WithSort(clientv3.SortByModRevision, clientv3.SortAscend)}
	opts = append(opts, clientv3.WithFromKey(), clientv3.WithKeysOnly())
	key := "\x00"
	resp, err := c.Get(context.Background(), key, opts...)
	if err != nil {
		return nil, err
	}
	return resp.Kvs, nil
}

func mustCreateConn(connEndpoint string) (*clientv3.Client, error) {
	cfg := clientv3.Config{
		Endpoints: []string{connEndpoint},
	}
	if !tls.Empty() || tls.TrustedCAFile != "" {
		cfgtls, err := tls.ClientConfig()
		if err != nil {
			lg.Error("bad tls config", zap.Error(err))
			return nil, errors.New("bad tls config")
		}
		cfg.TLS = cfgtls
	}

	if len(user) != 0 {
		username, password, err := getUsernamePassword(user)
		if err != nil {
			lg.Error("bad user information", zap.String("user", user), zap.Error(err))
			return nil, errors.New("bad user information")
		}
		cfg.Username = username
		cfg.Password = password
	}

	client, err := clientv3.New(cfg)

	grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr))

	if err != nil {
		lg.Error("dial error", zap.Error(err))
		return nil, errors.New("dial error")
	}

	return client, nil
}

func getUsernamePassword(usernameFlag string) (string, string, error) {
	if globalUserName != "" && globalPassword != "" {
		return globalUserName, globalPassword, nil
	}
	colon := strings.Index(usernameFlag, ":")
	if colon == -1 {
		// Prompt for the password.
		password, err := speakeasy.Ask("Password: ")
		if err != nil {
			return "", "", err
		}
		globalUserName = usernameFlag
		globalPassword = password
	} else {
		globalUserName = usernameFlag[:colon]
		globalPassword = usernameFlag[colon+1:]
	}
	return globalUserName, globalPassword, nil
}
