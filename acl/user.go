package acl

type User interface {
	Name() string
	Password() string
	Roles() []Role
	// Root returns true if this user is a root user.
	// A root user has a root role that can manage user resources,
	// including users and roles.
	Root() bool

	// Manage the User
	// Before calling these functions, the caller must check if it has root role.
	// For example, if A wants to add role to B, A must has root role. Then A
	// can call add role on B.
	AddRole(role string) error
	RemoveRole(role string) error
}

type rootUser string
