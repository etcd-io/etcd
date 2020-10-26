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
	"io/ioutil"
	"os"
	"testing"

	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
)

func TestEnableAuth(t *testing.T) {
	tdir, err := ioutil.TempDir(os.TempDir(), "auth-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tdir)
	cfg := NewConfig()
	cfg.Dir = tdir
	e, err := StartEtcd(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	client := v3client.New(e.Server)
	defer client.Close()

	_, err = client.RoleAdd(context.TODO(), "root")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.UserAdd(context.TODO(), "root", "root")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.UserGrantRole(context.TODO(), "root", "root")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.AuthEnable(context.TODO())
	if err != nil {
		t.Fatal(err)
	}
}
