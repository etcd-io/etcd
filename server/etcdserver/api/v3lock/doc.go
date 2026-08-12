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

// Package v3lock provides a v3 locking service from an etcdserver.
//
// Caution: the gRPC Lock service is not recommended for general use. Lock
// blocks on the server for as long as the caller is waiting to acquire the
// lock, reusing the auth token from the original request for internal
// calls made while it waits. If that token expires before the call
// returns, Lock fails with an auth error even though the caller never did
// anything wrong (see https://github.com/etcd-io/etcd/issues/17623).
// Prefer the client/v3/concurrency package directly where a client SDK is
// available; this service mainly exists for clients without a native SDK
// equivalent.
package v3lock
