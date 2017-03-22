// Copyright 2016 The etcd Authors
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

package auth

import (
	"github.com/coreos/etcd/auth/authpb"
	"github.com/coreos/etcd/mvcc/backend"
	"github.com/coreos/etcd/pkg/adt"
)

func getMergedPerms(tx backend.BatchTx, userName string) *unifiedRangePermissions {
	user := getUser(tx, userName)
	if user == nil {
		plog.Errorf("invalid user name %s", userName)
		return nil
	}

	readPerms := &adt.IntervalTree{}
	writePerms := &adt.IntervalTree{}

	for _, roleName := range user.Roles {
		role := getRole(tx, roleName)
		if role == nil {
			continue
		}

		for _, perm := range role.KeyPermission {
			var ivl adt.Interval

			if len(perm.RangeEnd) != 0 {
				ivl = adt.NewStringInterval(string(perm.Key), string(perm.RangeEnd))
			} else {
				ivl = adt.NewStringPoint(string(perm.Key))
			}

			switch perm.PermType {
			case authpb.READWRITE:
				readPerms.Insert(ivl, struct{}{})
				writePerms.Insert(ivl, struct{}{})

			case authpb.READ:
				readPerms.Insert(ivl, struct{}{})

			case authpb.WRITE:
				writePerms.Insert(ivl, struct{}{})
			}
		}
	}

	return &unifiedRangePermissions{
		readPerms:  readPerms,
		writePerms: writePerms,
	}
}

func checkKeyInterval(cachedPerms *unifiedRangePermissions, key, rangeEnd string, permtyp authpb.Permission_Type) bool {
	var tocheck *adt.IntervalTree

	switch permtyp {
	case authpb.READ:
		tocheck = cachedPerms.readPerms
	case authpb.WRITE:
		tocheck = cachedPerms.writePerms
	default:
		plog.Panicf("unknown auth type: %v", permtyp)
	}

	ivl := adt.NewStringInterval(key, rangeEnd)

	isContiguous := true
	var maxEnd, minBegin adt.Comparable

	tocheck.Visit(ivl, func(n *adt.IntervalValue) bool {
		if minBegin == nil {
			minBegin = n.Ivl.Begin
			maxEnd = n.Ivl.End
			return true
		}

		if maxEnd.Compare(n.Ivl.Begin) < 0 {
			isContiguous = false
			return false
		}

		if n.Ivl.End.Compare(maxEnd) > 0 {
			maxEnd = n.Ivl.End
		}

		return true
	})

	return isContiguous && maxEnd.Compare(ivl.End) >= 0 && minBegin.Compare(ivl.Begin) <= 0
}

func checkKeyPoint(cachedPerms *unifiedRangePermissions, key string, permtyp authpb.Permission_Type) bool {
	var tocheck *adt.IntervalTree

	switch permtyp {
	case authpb.READ:
		tocheck = cachedPerms.readPerms
	case authpb.WRITE:
		tocheck = cachedPerms.writePerms
	default:
		plog.Panicf("unknown auth type: %v", permtyp)
	}

	pt := adt.NewStringPoint(key)

	return tocheck.Contains(pt)
}

func (as *authStore) isRangeOpPermitted(tx backend.BatchTx, userName string, key, rangeEnd string, permtyp authpb.Permission_Type) bool {
	// assumption: tx is Lock()ed
	_, ok := as.rangePermCache[userName]
	if !ok {
		perms := getMergedPerms(tx, userName)
		if perms == nil {
			plog.Errorf("failed to create a unified permission of user %s", userName)
			return false
		}
		as.rangePermCache[userName] = perms
	}

	if len(rangeEnd) == 0 {
		return checkKeyPoint(as.rangePermCache[userName], key, permtyp)
	}

	return checkKeyInterval(as.rangePermCache[userName], key, rangeEnd, permtyp)
}

func (as *authStore) clearCachedPerm() {
	as.rangePermCache = make(map[string]*unifiedRangePermissions)
}

func (as *authStore) invalidateCachedPerm(userName string) {
	delete(as.rangePermCache, userName)
}

type unifiedRangePermissions struct {
	readPerms  *adt.IntervalTree
	writePerms *adt.IntervalTree
}
