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

// Package login1 provides integration with the systemd logind API.  See http://www.freedesktop.org/wiki/Software/systemd/logind/
package login1

import (
	"fmt"
	"os"
	"strconv"

	"github.com/godbus/dbus"
)

const (
	dbusInterface = "org.freedesktop.login1.Manager"
	dbusPath      = "/org/freedesktop/login1"
)

// Conn is a connection to systemds dbus endpoint.
type Conn struct {
	conn   *dbus.Conn
	object dbus.BusObject
}

// New establishes a connection to the system bus and authenticates.
func New() (*Conn, error) {
	c := new(Conn)

	if err := c.initConnection(); err != nil {
		return nil, err
	}

	return c, nil
}

// Close closes the dbus connection
func (c *Conn) Close() {
	if c == nil {
		return
	}

	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Conn) initConnection() error {
	var err error
	c.conn, err = dbus.SystemBusPrivate()
	if err != nil {
		return err
	}

	// Only use EXTERNAL method, and hardcode the uid (not username)
	// to avoid a username lookup (which requires a dynamically linked
	// libc)
	methods := []dbus.Auth{dbus.AuthExternal(strconv.Itoa(os.Getuid()))}

	err = c.conn.Auth(methods)
	if err != nil {
		c.conn.Close()
		return err
	}

	err = c.conn.Hello()
	if err != nil {
		c.conn.Close()
		return err
	}

	c.object = c.conn.Object("org.freedesktop.login1", dbus.ObjectPath(dbusPath))

	return nil
}

// Session object definition.
type Session struct {
	ID   string
	UID  uint32
	User string
	Seat string
	Path dbus.ObjectPath
}

// User object definition.
type User struct {
	UID  uint32
	Name string
	Path dbus.ObjectPath
}

func (s Session) toInterface() []interface{} {
	return []interface{}{s.ID, s.UID, s.User, s.Seat, s.Path}
}

func sessionFromInterfaces(session []interface{}) (*Session, error) {
	if len(session) < 5 {
		return nil, fmt.Errorf("invalid number of session fields: %d", len(session))
	}
	id, ok := session[0].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 0 to string")
	}
	uid, ok := session[1].(uint32)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 1 to uint32")
	}
	user, ok := session[2].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 2 to string")
	}
	seat, ok := session[3].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 2 to string")
	}
	path, ok := session[4].(dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 4 to ObjectPath")
	}

	ret := Session{ID: id, UID: uid, User: user, Seat: seat, Path: path}
	return &ret, nil
}

func userFromInterfaces(user []interface{}) (*User, error) {
	if len(user) < 3 {
		return nil, fmt.Errorf("invalid number of user fields: %d", len(user))
	}
	uid, ok := user[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("failed to typecast user field 0 to uint32")
	}
	name, ok := user[1].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast session field 1 to string")
	}
	path, ok := user[2].(dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("failed to typecast user field 2 to ObjectPath")
	}

	ret := User{UID: uid, Name: name, Path: path}
	return &ret, nil
}

// GetSession may be used to get the session object path for the session with the specified ID.
func (c *Conn) GetSession(id string) (dbus.ObjectPath, error) {
	var out interface{}
	if err := c.object.Call(dbusInterface+".GetSession", 0, id).Store(&out); err != nil {
		return "", err
	}

	ret, ok := out.(dbus.ObjectPath)
	if !ok {
		return "", fmt.Errorf("failed to typecast session to ObjectPath")
	}

	return ret, nil
}

// ListSessions returns an array with all current sessions.
func (c *Conn) ListSessions() ([]Session, error) {
	out := [][]interface{}{}
	if err := c.object.Call(dbusInterface+".ListSessions", 0).Store(&out); err != nil {
		return nil, err
	}

	ret := []Session{}
	for _, el := range out {
		session, err := sessionFromInterfaces(el)
		if err != nil {
			return nil, err
		}
		ret = append(ret, *session)
	}
	return ret, nil
}

// ListUsers returns an array with all currently logged in users.
func (c *Conn) ListUsers() ([]User, error) {
	out := [][]interface{}{}
	if err := c.object.Call(dbusInterface+".ListUsers", 0).Store(&out); err != nil {
		return nil, err
	}

	ret := []User{}
	for _, el := range out {
		user, err := userFromInterfaces(el)
		if err != nil {
			return nil, err
		}
		ret = append(ret, *user)
	}
	return ret, nil
}

// LockSession asks the session with the specified ID to activate the screen lock.
func (c *Conn) LockSession(id string) {
	c.object.Call(dbusInterface+".LockSession", 0, id)
}

// LockSessions asks all sessions to activate the screen locks. This may be used to lock any access to the machine in one action.
func (c *Conn) LockSessions() {
	c.object.Call(dbusInterface+".LockSessions", 0)
}

// TerminateSession forcibly terminate one specific session.
func (c *Conn) TerminateSession(id string) {
	c.object.Call(dbusInterface+".TerminateSession", 0, id)
}

// TerminateUser forcibly terminates all processes of a user.
func (c *Conn) TerminateUser(uid uint32) {
	c.object.Call(dbusInterface+".TerminateUser", 0, uid)
}

// Reboot asks logind for a reboot optionally asking for auth.
func (c *Conn) Reboot(askForAuth bool) {
	c.object.Call(dbusInterface+".Reboot", 0, askForAuth)
}

// Inhibit takes inhibition lock in logind.
func (c *Conn) Inhibit(what, who, why, mode string) (*os.File, error) {
	var fd dbus.UnixFD

	err := c.object.Call(dbusInterface+".Inhibit", 0, what, who, why, mode).Store(&fd)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), "inhibit"), nil
}

// Subscribe to signals on the logind dbus
func (c *Conn) Subscribe(members ...string) chan *dbus.Signal {
	for _, member := range members {
		c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
			fmt.Sprintf("type='signal',interface='org.freedesktop.login1.Manager',member='%s'", member))
	}
	ch := make(chan *dbus.Signal, 10)
	c.conn.Signal(ch)
	return ch
}

// PowerOff asks logind for a power off optionally asking for auth.
func (c *Conn) PowerOff(askForAuth bool) {
	c.object.Call(dbusInterface+".PowerOff", 0, askForAuth)
}
