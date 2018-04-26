// Copyright 2018 The etcd Authors
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

package etcdserverpb

import "fmt"

// InternalRaftStringer implements custom proto Stringer:
// redact password, shorten output(TODO).
type InternalRaftStringer struct {
	Request *InternalRaftRequest
}

func (as *InternalRaftStringer) String() string {
	switch {
	case as.Request.LeaseGrant != nil:
		return fmt.Sprintf("header:<%s> lease_grant:<ttl:%d-second id:%016x>",
			as.Request.Header.String(),
			as.Request.LeaseGrant.TTL,
			as.Request.LeaseGrant.ID,
		)
	case as.Request.LeaseRevoke != nil:
		return fmt.Sprintf("header:<%s> lease_revoke:<id:%016x>",
			as.Request.Header.String(),
			as.Request.LeaseRevoke.ID,
		)
	case as.Request.Authenticate != nil:
		return fmt.Sprintf("header:<%s> authenticate:<name:%s simple_token:%s>",
			as.Request.Header.String(),
			as.Request.Authenticate.Name,
			as.Request.Authenticate.SimpleToken,
		)
	case as.Request.AuthUserAdd != nil:
		return fmt.Sprintf("header:<%s> auth_user_add:<name:%s>",
			as.Request.Header.String(),
			as.Request.AuthUserAdd.Name,
		)
	case as.Request.AuthUserChangePassword != nil:
		return fmt.Sprintf("header:<%s> auth_user_change_password:<name:%s>",
			as.Request.Header.String(),
			as.Request.AuthUserChangePassword.Name,
		)
	default:
		// nothing to redact
	}
	return as.Request.String()
}
