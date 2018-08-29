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

package rpcpb

import (
	"fmt"
	"reflect"
	"strings"

	"go.etcd.io/etcd/embed"
	"go.etcd.io/etcd/pkg/transport"
	"go.etcd.io/etcd/pkg/types"
)

var etcdFields = []string{
	"Name",
	"DataDir",
	"WALDir",

	"HeartbeatIntervalMs",
	"ElectionTimeoutMs",

	"ListenClientURLs",
	"AdvertiseClientURLs",
	"ClientAutoTLS",
	"ClientCertAuth",
	"ClientCertFile",
	"ClientKeyFile",
	"ClientTrustedCAFile",

	"ListenPeerURLs",
	"AdvertisePeerURLs",
	"PeerAutoTLS",
	"PeerClientCertAuth",
	"PeerCertFile",
	"PeerKeyFile",
	"PeerTrustedCAFile",

	"InitialCluster",
	"InitialClusterState",
	"InitialClusterToken",

	"SnapshotCount",
	"QuotaBackendBytes",

	"PreVote",
	"InitialCorruptCheck",

	"Logger",
	"LogOutputs",
	"Debug",
}

// Flags returns etcd flags in string slice.
func (e *Etcd) Flags() (fs []string) {
	tp := reflect.TypeOf(*e)
	vo := reflect.ValueOf(*e)
	for _, name := range etcdFields {
		field, ok := tp.FieldByName(name)
		if !ok {
			panic(fmt.Errorf("field %q not found", name))
		}
		fv := reflect.Indirect(vo).FieldByName(name)
		var sv string
		switch fv.Type().Kind() {
		case reflect.String:
			sv = fv.String()
		case reflect.Slice:
			n := fv.Len()
			sl := make([]string, n)
			for i := 0; i < n; i++ {
				sl[i] = fv.Index(i).String()
			}
			sv = strings.Join(sl, ",")
		case reflect.Int64:
			sv = fmt.Sprintf("%d", fv.Int())
		case reflect.Bool:
			sv = fmt.Sprintf("%v", fv.Bool())
		default:
			panic(fmt.Errorf("field %q (%v) cannot be parsed", name, fv.Type().Kind()))
		}

		fname := field.Tag.Get("yaml")

		// TODO: remove this
		if fname == "initial-corrupt-check" {
			fname = "experimental-" + fname
		}

		if sv != "" {
			fs = append(fs, fmt.Sprintf("--%s=%s", fname, sv))
		}
	}
	return fs
}

// EmbedConfig returns etcd embed.Config.
func (e *Etcd) EmbedConfig() (cfg *embed.Config, err error) {
	var lcURLs types.URLs
	lcURLs, err = types.NewURLs(e.ListenClientURLs)
	if err != nil {
		return nil, err
	}
	var acURLs types.URLs
	acURLs, err = types.NewURLs(e.AdvertiseClientURLs)
	if err != nil {
		return nil, err
	}
	var lpURLs types.URLs
	lpURLs, err = types.NewURLs(e.ListenPeerURLs)
	if err != nil {
		return nil, err
	}
	var apURLs types.URLs
	apURLs, err = types.NewURLs(e.AdvertisePeerURLs)
	if err != nil {
		return nil, err
	}

	cfg = embed.NewConfig()
	cfg.Name = e.Name
	cfg.Dir = e.DataDir
	cfg.WalDir = e.WALDir
	cfg.TickMs = uint(e.HeartbeatIntervalMs)
	cfg.ElectionMs = uint(e.ElectionTimeoutMs)

	cfg.LCUrls = lcURLs
	cfg.ACUrls = acURLs
	cfg.ClientAutoTLS = e.ClientAutoTLS
	cfg.ClientTLSInfo = transport.TLSInfo{
		ClientCertAuth: e.ClientCertAuth,
		CertFile:       e.ClientCertFile,
		KeyFile:        e.ClientKeyFile,
		TrustedCAFile:  e.ClientTrustedCAFile,
	}

	cfg.LPUrls = lpURLs
	cfg.APUrls = apURLs
	cfg.PeerAutoTLS = e.PeerAutoTLS
	cfg.PeerTLSInfo = transport.TLSInfo{
		ClientCertAuth: e.PeerClientCertAuth,
		CertFile:       e.PeerCertFile,
		KeyFile:        e.PeerKeyFile,
		TrustedCAFile:  e.PeerTrustedCAFile,
	}

	cfg.InitialCluster = e.InitialCluster
	cfg.ClusterState = e.InitialClusterState
	cfg.InitialClusterToken = e.InitialClusterToken

	cfg.SnapshotCount = uint64(e.SnapshotCount)
	cfg.QuotaBackendBytes = e.QuotaBackendBytes

	cfg.PreVote = e.PreVote
	cfg.ExperimentalInitialCorruptCheck = e.InitialCorruptCheck

	cfg.Logger = e.Logger
	cfg.LogOutputs = e.LogOutputs
	cfg.Debug = e.Debug

	return cfg, nil
}
