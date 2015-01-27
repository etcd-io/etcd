// Copyright 2015 CoreOS, Inc.
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

package client

import (
	"errors"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

var (
	ErrTimeout          = context.DeadlineExceeded
	ErrCanceled         = context.Canceled
	ErrUnavailable      = errors.New("client: no available etcd endpoints")
	ErrNoLeader         = errors.New("client: no leader")
	ErrNoEndpoints      = errors.New("no endpoints available")
	ErrTooManyRedirects = errors.New("too many redirects")

	ErrKeyNoExist = errors.New("client: key does not exist")
	ErrKeyExists  = errors.New("client: key already exists")

	DefaultRequestTimeout = 5 * time.Second
	DefaultMaxRedirects   = 10
)
