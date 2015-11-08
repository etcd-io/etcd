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

package lease

import (
	"reflect"
	"testing"
	"time"
)

// TestLessorGrant ensures Lessor can grant wanted lease.
// The granted lease should have a unique ID with a term
// that is greater than minLeaseTerm.
func TestLessorGrant(t *testing.T) {
	le := NewLessor(1)

	l := le.Grant(time.Now().Add(time.Second))
	gl := le.get(l.id)

	if !reflect.DeepEqual(gl, l) {
		t.Errorf("lease = %v, want %v", gl, l)
	}
	if l.expiry.Sub(time.Now()) < minLeaseTerm-time.Second {
		t.Errorf("term = %v, want at least %v", l.expiry.Sub(time.Now()), minLeaseTerm-time.Second)
	}

	nl := le.Grant(time.Now().Add(time.Second))
	if nl.id == l.id {
		t.Errorf("new lease.id = %x, want != %x", nl.id, l.id)
	}
}

// TestLessorRevoke ensures Lessor can revoke a lease.
// The revoked lease cannot be got from Lessor again.
func TestLessorRevoke(t *testing.T) {
	le := NewLessor(1)

	// grant a lease with long term (100 seconds) to
	// avoid early termination during the test.
	l := le.Grant(time.Now().Add(100 * time.Second))

	err := le.Revoke(l.id)
	if err != nil {
		t.Fatal("failed to revoke lease:", err)
	}

	if le.get(l.id) != nil {
		t.Errorf("got revoked lease %x", l.id)
	}
}

// TestLessorRenew ensures Lessor can renew an existing lease.
func TestLessorRenew(t *testing.T) {
	le := NewLessor(1)
	l := le.Grant(time.Now().Add(5 * time.Second))

	le.Renew(l.id, time.Now().Add(100*time.Second))
	l = le.get(l.id)

	if l.expiry.Sub(time.Now()) < 95*time.Second {
		t.Errorf("failed to renew the lease for 100 seconds")
	}
}
