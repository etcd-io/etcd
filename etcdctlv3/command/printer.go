// Copyright 2016 CoreOS, Inc.
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
	"fmt"

	v3 "github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type printer interface {
	Del(v3.DeleteResponse)
	Get(v3.GetResponse)
	Put(v3.PutResponse)
	Txn(v3.TxnResponse)
	Watch(v3.WatchResponse)
}

type simplePrinter struct {
	isHex bool
}

func (s *simplePrinter) Del(v3.DeleteResponse) {
	// TODO: add number of key removed into the response of delete.
	// TODO: print out the number of removed keys.
	fmt.Println(0)
}

func (s *simplePrinter) Get(resp v3.GetResponse) {
	for _, kv := range resp.Kvs {
		printKV(s.isHex, kv)
	}
}

func (s *simplePrinter) Put(r v3.PutResponse) { fmt.Println("OK") }

func (s *simplePrinter) Txn(resp v3.TxnResponse) {
	if resp.Succeeded {
		fmt.Println("SUCCESS")
	} else {
		fmt.Println("FAILURE")
	}

	for _, r := range resp.Responses {
		fmt.Println("")
		switch v := r.Response.(type) {
		case *pb.ResponseUnion_ResponseDeleteRange:
			s.Del((v3.DeleteResponse)(*v.ResponseDeleteRange))
		case *pb.ResponseUnion_ResponsePut:
			s.Put((v3.PutResponse)(*v.ResponsePut))
		case *pb.ResponseUnion_ResponseRange:
			s.Get(((v3.GetResponse)(*v.ResponseRange)))
		default:
			fmt.Printf("unexpected response %+v\n", r)
		}
	}
}

func (s *simplePrinter) Watch(resp v3.WatchResponse) {
	for _, e := range resp.Events {
		fmt.Println(e.Type)
		printKV(s.isHex, e.Kv)
	}
}
