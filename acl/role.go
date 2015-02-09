package acl

type Role interface {
	Name() string
	// ACL specifics the capacity on the key-value resources.
	ACL() ACL
	// Root returns true if the role is a root role.
	// root role has the capacity to manage roles and users
	Root() bool
}

type rootRole string
