// Copyright 2017 The etcd Authors
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

// Package leasing is a clientv3 wrapper that provides the client exclusive write access to a key by acquiring a lease and be lineraizably
// served locally. This leasing layer can either directly wrap the etcd client or
// it can be exposed through the etcd grace proxy server, granting multiple clients write access.
//
// First, create a leasing client interface:
//
// 		leasingCli,error = leasing.NewKV(cli.KV, "leasing-prefix")
// 		if error != nil {
//				//handle error
// 		}
//
// The first range request acquires the lease by adding the leasing key ("leasing-prefix"/key) on the server and stores the key locally.
// Further linearized read requests using 'cli.leasing' will be served locally as long as the lease exists:
// 		cli.Put(context.TODO(), "abc", "123")
//
// Lease Acquisition:
//		leasingCli.Get(context.TODO(), "abc")
//
// Local reads:
//		resp,_ := leasingCli.Get(context.TODO(), "abc")
//		fmt.Printf("%s\n", resp.Kvs[0].Value)
//		//Output: 123 (served locally)
//
// Lease Revocation:
// If a client writes to the key owned by the leasing client,then the leasing client gives up its lease allowing the client to modify the key.
//		cli.Put(context.TODO(), "abc", "456")
//		resp, _ = leasingCli.Get("abc")
//		fmt.Printf("%s\n", resp.Kvs[0].Value)
//		// Output: 456  (fetched from server)
//
package leasing
