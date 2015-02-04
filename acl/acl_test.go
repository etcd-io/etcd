package acl

import "testing"

func TestACLRead(t *testing.T) {
	readp := []byte("foobar")
	acl := newTestEmpty()
	acl.GrantRead(readp)

	for i := 0; i < len(readp); i++ {
		r := acl.Read(readp[:i])
		if r == true {
			t.Errorf("#%d.deny: r = %t, want %t", i, r, false)
		}
	}

	for i := 0; i < len(readp); i++ {
		r := acl.Read(append(readp, readp[:i]...))
		if r == false {
			t.Errorf("#%d.allow: r = %t, want %t", i, r, true)
		}
	}
}

func TestACLWrite(t *testing.T) {
	writep := []byte("foobar")
	acl := newTestEmpty()
	acl.GrantWrite(writep)

	for i := 0; i < len(writep); i++ {
		w := acl.Write(writep[:i])
		if w == true {
			t.Errorf("#%d.deny: w = %t, want %t", i, w, false)
		}
	}

	for i := 0; i < len(writep); i++ {
		w := acl.Write(append(writep, writep[:i]...))
		if w == false {
			t.Errorf("#%d.allow: w = %t, want %t", i, w, true)
		}
	}
}

func TestACLManage(t *testing.T) {
	managep := []byte("foobar")
	acl := newTestEmpty()
	acl.GrantManage(managep)

	for i := 0; i < len(managep); i++ {
		m := acl.Manage(managep[:i])
		if m == true {
			t.Errorf("#%d.deny: m = %t, want %t", i, m, false)
		}
	}

	for i := 0; i < len(managep); i++ {
		m := acl.Manage(append(managep, managep[:i]...))
		if m == false {
			t.Errorf("#%d.allow: m = %t, want %t", i, m, true)
		}
	}
}

func TestACLRevoke(t *testing.T) {
	readp := []byte("foobar")
	acl := newTestEmpty()
	acl.GrantRead(readp)
	if !acl.Read(readp) {
		t.Errorf("r = %t, want %t", false, true)
	}

	acl.RevokeRead(readp)
	if acl.Read(readp) {
		t.Errorf("r = %t, want %t", true, false)
	}
}

func newTestEmpty() ACL {
	return &acl{}
}
