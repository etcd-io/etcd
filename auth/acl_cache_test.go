package auth

import (
	"math"
	"testing"

	"github.com/coreos/etcd/auth/authpb"
)

func TestAclEmpty(t *testing.T) {
	ac := newAclCache(0, []*authpb.AclEntry{})
	if res := ac.GetRights("/a/b/c"); res != math.MaxUint32 {
		t.Errorf("expected MaxUint32 got %v", res)
	}
}

func TestAclSingleEntry(t *testing.T) {
	ac := newAclCache(0, []*authpb.AclEntry{&authpb.AclEntry{"/a", 0x8, 0}})
	if res := ac.GetRights("/c/test1"); res != 0 {
		t.Errorf("expected 0 got %v", res)
	}
	if res := ac.GetRights("/a/"); res != 0x8 {
		t.Errorf("expected 0x8 got %v", res)
	}
	if res := ac.GetRights("/a/b/c"); res != 0x8 {
		t.Errorf("expected 0x8 got %v", res)
	}
}

func TestAclMultipleEntries(t *testing.T) {
	ac := newAclCache(0, []*authpb.AclEntry{
		&authpb.AclEntry{"/a", 0x7, 0},
		&authpb.AclEntry{"/a/b/c/d", 0x8, 0x1},
		&authpb.AclEntry{"/a/b/c/d/e", 0x1, 0x2},
	})
	if res := ac.GetRights("/c/test1"); res != 0 {
		t.Errorf("expected 0 got %v", res)
	}
	if res := ac.GetRights("/a/b/c/test1"); res != 0x7 {
		t.Errorf("expected 0x7 got %v", res)
	}
	if res := ac.GetRights("/a/b/c/d/test1"); res != 0xE {
		t.Errorf("expected 0xE got %v", res)
	}
	if res := ac.GetRights("/a/b/c/d/e/fgfgf/test2"); res != 0xD {
		t.Errorf("expected 0xD got %v", res)
	}
	if res := ac.GetRights("/a/b/c/d/e"); res != 0xE {
		t.Errorf("expected 0xE got %v", res)
	}
}
