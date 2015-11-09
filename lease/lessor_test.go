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
	le := NewLessor(1, &fakeDeleteable{})

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
// The items in the revoked lease should be removed from
// the DeleteableKV.
// The revoked lease cannot be got from Lessor again.
func TestLessorRevoke(t *testing.T) {
	fd := &fakeDeleteable{}
	le := NewLessor(1, fd)

	// grant a lease with long term (100 seconds) to
	// avoid early termination during the test.
	l := le.Grant(time.Now().Add(100 * time.Second))

	items := []leaseItem{
		{"foo", ""},
		{"bar", "zar"},
	}

	err := le.Attach(l.id, items)
	if err != nil {
		t.Fatalf("failed to attach items to the lease: %v", err)
	}

	err = le.Revoke(l.id)
	if err != nil {
		t.Fatal("failed to revoke lease:", err)
	}

	if le.get(l.id) != nil {
		t.Errorf("got revoked lease %x", l.id)
	}

	wdeleted := []string{"foo_", "bar_zar"}
	if !reflect.DeepEqual(fd.deleted, wdeleted) {
		t.Errorf("deleted= %v, want %v", fd.deleted, wdeleted)
	}
}

// TestLessorRenew ensures Lessor can renew an existing lease.
func TestLessorRenew(t *testing.T) {
	le := NewLessor(1, &fakeDeleteable{})
	l := le.Grant(time.Now().Add(5 * time.Second))

	le.Renew(l.id, time.Now().Add(100*time.Second))
	l = le.get(l.id)

	if l.expiry.Sub(time.Now()) < 95*time.Second {
		t.Errorf("failed to renew the lease for 100 seconds")
	}
}

type fakeDeleteable struct {
	deleted []string
}

func (fd *fakeDeleteable) DeleteRange(key, end []byte) (int64, int64) {
	fd.deleted = append(fd.deleted, string(key)+"_"+string(end))
	return 0, 0
}
