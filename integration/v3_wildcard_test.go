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

package integration

import (
	"golang.org/x/net/context"
	"testing"
)

func TestWildcardPathQueries(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.RandClient()
	if _, err := cli.Put(context.TODO(), "/home/joe/car", "margaret"); err != nil {
		t.Fatal(err)
	}
	if _, err := cli.Put(context.TODO(), "/home/joe/boat", "felicity"); err != nil {
		t.Fatal(err)
	}

	fetched, err := cli.Get(context.TODO(), "/home/joe/*")
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Count != 2 {
		t.Fatalf("expected 2, got %v", fetched.Count)
	}

	// escaped stars do nothing
	esc, err := cli.Delete(context.TODO(), `/home/joe/\*`)
	if err != nil {
		t.Fatal(err)
	}
	if esc.Deleted != 0 {
		t.Fatalf("expected 0, got %v", esc.Deleted)
	}

	del, err := cli.Delete(context.TODO(), `/home/joe/*`)
	if err != nil {
		t.Fatal(err)
	}
	if del.Deleted != 2 {
		t.Fatalf("expected 2, got %v", del.Deleted)
	}

	// read back a key that ends in star
	if _, err = cli.Put(context.TODO(), "abc*", "felicity"); err != nil {
		t.Fatal(err)
	}

	get, err := cli.Get(context.TODO(), `abc\*`)
	if err != nil {
		t.Fatal(err)
	}
	if get.Count != 1 {
		t.Fatalf("expected 1, got %v", get.Count)
	}
}
