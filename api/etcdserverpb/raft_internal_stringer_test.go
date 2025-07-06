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

package etcdserverpb_test

import (
	"testing"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

// TestInvalidGoYypeIntPanic tests conditions that caused
// panic: invalid Go type int for field k8s_io.kubernetes.vendor.go_etcd_io.etcd.etcdserver.etcdserverpb.loggablePutRequest.value_size
// See https://github.com/kubernetes/kubernetes/issues/91937 for more details
func TestInvalidGoTypeIntPanic(t *testing.T) {
	result := pb.NewLoggablePutRequest(&pb.PutRequest{}).String()
	if result != "" {
		t.Errorf("Got result: %s, expected empty string", result)
	}
}
