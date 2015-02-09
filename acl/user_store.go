package acl

import "errors"

var (
	ErrUserNotExist = errors.New("acl: user does not exist")
	ErrUserExist    = errors.New("acl: user already exists")
)

type UserStore interface {
	GetUser(name string) (User, error)

	// Before calling the theses functions, the caller must check if it has root role.
	AddUser(name, password string) error
	RemoveUser(name string) error
}
