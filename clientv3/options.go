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

package clientv3

import (
	"math"

	"google.golang.org/grpc"
)

var (
	// client-side handling retrying of request failures where data was not written to the wire or
	// where server indicates it did not process the data. gPRC default is default is "FailFast(true)"
	// but for etcd we default to "FailFast(false)" to minimize client request error responses due to
	// transident failures.
	defaultFailFast = grpc.FailFast(false)

	// client-side request send limit, gRPC default is math.MaxInt32
	// Make sure that "client-side send limit < server-side default send/recv limit"
	// Same value as "embed.DefaultMaxRequestBytes" plus gRPC overhead bytes
	defaultMaxCallSendMsgSize = grpc.MaxCallSendMsgSize(2 * 1024 * 1024)

	// client-side response receive limit, gRPC default is 4MB
	// Make sure that "client-side receive limit >= server-side default send/recv limit"
	// because range response can easily exceed request send limits
	// Default to math.MaxInt32; writes exceeding server-side send limit fails anyway
	defaultMaxCallRecvMsgSize = grpc.MaxCallRecvMsgSize(math.MaxInt32)
)

// defaultCallOpts defines a list of default "gRPC.CallOption".
// Some options are exposed to "clientv3.Config".
// Defaults will be overridden by the settings in "clientv3.Config".
var defaultCallOpts = []grpc.CallOption{defaultFailFast, defaultMaxCallSendMsgSize, defaultMaxCallRecvMsgSize}

// MaxLeaseTTL is the maximum lease TTL value
const MaxLeaseTTL = 9000000000
