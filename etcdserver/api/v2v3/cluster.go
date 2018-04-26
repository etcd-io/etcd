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

package v2v3

import (
	"context"
	"time"

	"github.com/coreos/etcd/etcdserver/membership"
	"github.com/coreos/etcd/pkg/types"

	"github.com/coreos/go-semver/semver"
)

func (s *v2v3Server) ID() types.ID {
	// TODO: use an actual member ID
	return types.ID(0xe7cd2f00d)
}
func (s *v2v3Server) ClientURLs() []string { panic("STUB") }

func (s *v2v3Server) Members() []*membership.Member {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	resp, err := s.c.MemberList(ctx)
	if err != nil {
		// TODO: how can we upport errors here?
		panic("STUB")
	}
	return v3MembersToMembership(resp.Members)
}

func (s *v2v3Server) Member(id types.ID) *membership.Member { panic("STUB") }
func (s *v2v3Server) Version() *semver.Version              { panic("STUB") }
