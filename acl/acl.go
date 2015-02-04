package acl

import (
	"bytes"
	"log"
)

const (
	read   = "read"
	write  = "write"
	manage = "manage"
)

type ACL interface {
	// Read checks the permission to read the prefix.
	// Read permission can read the value of the prefix as a key or list the keys under prefix.
	Read(prefix []byte) bool
	// Write checks the permission to write the prefix.
	// Write permission can write a key under the given prefix (including the prefix as a key).
	Write(prefix []byte) bool
	// Manage checks the permission to grant /revoke permission for the prefix.
	Manage(prefix []byte) bool

	// Manage the ACL
	// Before call these functions, the caller must check the management permission on its own.
	// For example, A wants to grant B read permission for prefix "foo". A must check its own manage
	// permission before calling GrantRead("foo") on B.
	GrantRead(prefix []byte)
	GrantWrite(prefix []byte)
	GrantManage(prefix []byte)
	RevokeRead(prefix []byte)
	RevokeWrite(prefix []byte)
	RevokeManage(prefix []byte)
	Clear()
}

type acl struct {
	write  [][]byte
	read   [][]byte
	manage [][]byte

	parent ACL
}

func (a *acl) Read(prefix []byte) bool {
	if a.check(read, prefix) {
		return true
	}
	if a.parent != nil {
		return a.parent.Read(prefix)
	}
	return false
}

func (a *acl) Write(prefix []byte) bool {
	if a.check(write, prefix) {
		return true
	}
	if a.parent != nil {
		return a.parent.Write(prefix)
	}
	return false
}

func (a *acl) Manage(prefix []byte) bool {
	if a.check(manage, prefix) {
		return true
	}
	if a.parent != nil {
		return a.parent.Manage(prefix)
	}
	return false
}

func (a *acl) GrantRead(prefix []byte)   { a.grant(read, prefix) }
func (a *acl) GrantWrite(prefix []byte)  { a.grant(write, prefix) }
func (a *acl) GrantManage(prefix []byte) { a.grant(manage, prefix) }

func (a *acl) RevokeRead(prefix []byte)   { a.revoke(read, prefix) }
func (a *acl) RevokeWrite(prefix []byte)  { a.revoke(write, prefix) }
func (a *acl) RevokeManage(prefix []byte) { a.revoke(manage, prefix) }

func (a *acl) Clear() {
	a.read = nil
	a.write = nil
	a.manage = nil
}

func (a *acl) check(op string, prefix []byte) bool {
	list := a.list(op)
	for _, pp := range list {
		if len(pp) == 0 {
			continue
		}
		if bytes.HasPrefix(prefix, pp) {
			return true
		}
	}
	return false
}

func (a *acl) grant(op string, prefix []byte) {
	list := a.list(op)
	for _, pp := range list {
		if bytes.Equal(prefix, pp) {
			return
		}
	}

	switch op {
	case read:
		a.read = append(a.read, prefix)
	case write:
		a.write = append(a.write, prefix)
	case manage:
		a.manage = append(a.manage, prefix)
	default:
		log.Panicf("acl: unexpected check operation %s", op)
	}
	return
}

func (a *acl) revoke(op string, prefix []byte) {
	list := a.list(op)
	for i, pp := range list {
		if bytes.Equal(prefix, pp) {
			list[i] = nil
		}
	}
}

func (a *acl) list(op string) [][]byte {
	switch op {
	case read:
		return a.read
	case write:
		return a.write
	case manage:
		return a.manage
	default:
		log.Panicf("acl: unexpected check operation %s", op)
	}
	return nil
}
