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
	"LogLevel",

	"SocketReuseAddress",
	"SocketReusePort",
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
