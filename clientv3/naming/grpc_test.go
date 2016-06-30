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

package naming

import (
	"reflect"
	"testing"

	"google.golang.org/grpc/naming"

	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestGRPCResolver(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	r := GRPCResolver{
		Client: clus.RandClient(),
	}

	w, err := r.Resolve("foo")
	if err != nil {
		t.Fatal("failed to resolve foo", err)
	}
	defer w.Close()

	err = r.Add("foo", "127.0.0.1", "metadata")
	if err != nil {
		t.Fatal("failed to add foo", err)
	}

	us, err := w.Next()
	if err != nil {
		t.Fatal("failed to get udpate", err)
	}

	wu := &naming.Update{
		Op:       naming.Add,
		Addr:     "127.0.0.1",
		Metadata: "metadata",
	}

	if !reflect.DeepEqual(us[0], wu) {
		t.Fatalf("up = %#v, want %#v", us[0], wu)
	}

	err = r.Delete("foo")

	us, err = w.Next()
	if err != nil {
		t.Fatal("failed to get udpate", err)
	}

	wu = &naming.Update{
		Op: naming.Delete,
	}

	if !reflect.DeepEqual(us[0], wu) {
		t.Fatalf("up = %#v, want %#v", us[0], wu)
	}
}
