// Copyright 2020 The etcd Authors
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

package embed

import (
	"context"
	"os"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

type dummy struct {
	callCount int
}

func (d *dummy) dummyUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		d.callCount++
		return handler(ctx, req)
	}
}

func TestInterceptor(t *testing.T) {
	tdir, err := os.MkdirTemp(os.TempDir(), "interceptor-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)
	cfg := NewConfig()
	cfg.Dir = tdir
	d := &dummy{}

	cfg.GRPCInterceptor = d.dummyUnaryServerInterceptor()
	e, err := StartEtcd(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	client, err := clientv3.New(clientv3.Config{Endpoints: []string{cfg.LCUrls[0].String()}})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	_, err = client.Put(context.Background(), "foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if d.callCount != 1 {
		t.Fatalf("dummyUnaryServerInterceptor not work call count: %d", d.callCount)
	}
}
