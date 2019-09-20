package grpcproxy

import (
	"testing"

	"github.com/coreos/etcd/auth/authpb"
)

func TestAclToString(t *testing.T) {
	s1 := aclToString([]*authpb.AclEntry{
		{Path: "", RightsSet: 0, RightsUnset: 0},
		{Path: "/channels", RightsSet: 1, RightsUnset: 0},
		{Path: "/channels/abc123", RightsSet: 2, RightsUnset: 0},
	})
	s2 := aclToString([]*authpb.AclEntry{
		{Path: "", RightsSet: 0, RightsUnset: 0},
		{Path: "/channels", RightsSet: 1, RightsUnset: 0},
		{Path: "/channels/abc123", RightsSet: 2, RightsUnset: 1},
	})

	if s1 == s2 {
		t.Errorf("acl1 check failed")
	}

	s1 = aclToString([]*authpb.AclEntry{})
	s2 = aclToString([]*authpb.AclEntry{{}})

	if s1 == s2 {
		t.Errorf("acl2 check failed")
	}

	s1 = aclToString(nil)
	s2 = aclToString([]*authpb.AclEntry{})

	if s1 != s2 {
		t.Errorf("acl3 check failed")
	}

	s1 = aclToString([]*authpb.AclEntry{
		{Path: "", RightsSet: 0, RightsUnset: 0},
		{Path: "/channels/abc123", RightsSet: 2, RightsUnset: 0},
		{Path: "/channels", RightsSet: 1, RightsUnset: 0},
	})
	s2 = aclToString([]*authpb.AclEntry{
		{Path: "", RightsSet: 0, RightsUnset: 0},
		{Path: "/channels", RightsSet: 1, RightsUnset: 0},
		{Path: "/channels/abc123", RightsSet: 2, RightsUnset: 0},
	})

	if s1 != s2 {
		t.Errorf("acl4 check failed")
	}
}
