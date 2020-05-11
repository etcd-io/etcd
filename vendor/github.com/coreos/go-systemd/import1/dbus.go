// Copyright 2019 CoreOS, Inc.
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

// Package import1 provides integration with the systemd-importd API.  See https://www.freedesktop.org/wiki/Software/systemd/importd/
// Note: Requires systemd v231 or higher
package import1

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/godbus/dbus"
)

const (
	dbusInterface = "org.freedesktop.import1.Manager"
	dbusPath      = "/org/freedesktop/import1"
)

// Conn is a connection to systemds dbus endpoint.
type Conn struct {
	conn   *dbus.Conn
	object dbus.BusObject
}

// Transfer is an object in dbus for an import, export or download operation.
type Transfer struct {
	Id   uint32          // The numeric transfer ID of the transfer object
	Path dbus.ObjectPath // The dbus objectPath for the transfer
}

// TransferStatus is the status for an import, export or download operation.
type TransferStatus struct {
	Id       uint32  // The numeric transfer ID of the transfer object
	Local    string  // The local container name of this transfer
	Remote   string  // The remote source (in case of download: the URL, in case of import/export a string describing the file descriptor passed in)
	Type     string  // The type of operation
	Verify   string  // The selected verification setting, and is only defined for download operations
	Progress float64 // The current progress of the transfer, as a value between 0.0 and 1.0
}

// New establishes a connection to the system bus and authenticates.
// Note: systemd-importd will be activated via D-Bus, we don't need to check service status.
func New() (*Conn, error) {
	c := new(Conn)

	if err := c.initConnection(); err != nil {
		return nil, err
	}

	return c, nil
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

	c.object = c.conn.Object("org.freedesktop.import1", dbus.ObjectPath(dbusPath))

	return nil
}

// getResult will return:
//   - transfer object (*Transfer)
//   - err (error)
func (c *Conn) getResult(method string, args ...interface{}) (*Transfer, error) {
	result := c.object.Call(fmt.Sprintf("%s.%s", dbusInterface, method), 0, args...)
	if result.Err != nil {
		return nil, result.Err
	}

	if len(result.Body) < 2 {
		return nil, fmt.Errorf("invalid number of result fields: %v", result.Body)
	}

	ok := false
	transfer := &Transfer{}

	transfer.Id, ok = result.Body[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("unable to convert dbus response '%v' to uint32", result.Body[0])
	}

	transfer.Path, ok = result.Body[1].(dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("unable to convert dbus response '%v' to dbus.ObjectPath", result.Body[1])
	}
	return transfer, nil
}

// ImportTar imports a tar into systemd-importd.
func (c *Conn) ImportTar(
	f *os.File, local_name string, force, read_only bool,
) (*Transfer, error) {
	return c.getResult("ImportTar", dbus.UnixFD(f.Fd()), local_name, force, read_only)
}

// ImportRaw imports a raw image into systemd-importd.
func (c *Conn) ImportRaw(
	f *os.File, local_name string, force, read_only bool,
) (*Transfer, error) {
	return c.getResult("ImportRaw", dbus.UnixFD(f.Fd()), local_name, force, read_only)
}

// ExportTar exports a tar from systemd-importd.
func (c *Conn) ExportTar(
	local_name string, f *os.File, format string,
) (*Transfer, error) {
	return c.getResult("ExportTar", local_name, dbus.UnixFD(f.Fd()), format)
}

// ExportRaw exports a raw image from systemd-importd.
func (c *Conn) ExportRaw(
	local_name string, f *os.File, format string,
) (*Transfer, error) {
	return c.getResult("ExportRaw", local_name, dbus.UnixFD(f.Fd()), format)
}

// PullTar pulls a tar into systemd-importd.
func (c *Conn) PullTar(
	url, local_name, verify_mode string, force bool,
) (*Transfer, error) {
	return c.getResult("PullTar", url, local_name, verify_mode, force)
}

// PullRaw pulls a raw image into systemd-importd.
func (c *Conn) PullRaw(
	url, local_name, verify_mode string, force bool,
) (*Transfer, error) {
	return c.getResult("PullRaw", url, local_name, verify_mode, force)
}

// ListTransfers will list ongoing import, export or download operations.
func (c *Conn) ListTransfers() ([]TransferStatus, error) {
	result := make([][]interface{}, 0)
	if err := c.object.Call(dbusInterface+".ListTransfers", 0).Store(&result); err != nil {
		return nil, err
	}

	transfers := make([]TransferStatus, 0)
	for _, v := range result {
		transfer, err := transferFromInterfaces(v)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, *transfer)
	}

	return transfers, nil
}

// CancelTransfer will cancel an ongoing import, export or download operations.
func (c *Conn) CancelTransfer(transfer_id uint32) error {
	return c.object.Call(dbusInterface+".CancelTransfer", 0, transfer_id).Err
}

func transferFromInterfaces(transfer []interface{}) (*TransferStatus, error) {
	// Verify may be not defined in response.
	if len(transfer) < 5 {
		return nil, fmt.Errorf("invalid number of transfer fields: %d", len(transfer))
	}

	ok := false
	ret := &TransferStatus{}

	ret.Id, ok = transfer[0].(uint32)
	if !ok {
		return nil, fmt.Errorf("failed to typecast transfer field 0 to uint32")
	}
	ret.Local, ok = transfer[1].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast transfer field 1 to string")
	}
	ret.Remote, ok = transfer[2].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast transfer field 2 to string")
	}
	ret.Type, ok = transfer[3].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast transfer field 3 to string")
	}
	// Verify is only defined for download operations.
	// If operation is not pull, we should ignore Verify field.
	if !strings.HasPrefix(ret.Type, "pull-") {
		ret.Progress, ok = transfer[4].(float64)
		if !ok {
			return nil, fmt.Errorf("failed to typecast transfer field 4 to float64")
		}
		return ret, nil
	}

	ret.Verify, ok = transfer[4].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast transfer field 4 to string")
	}
	ret.Progress, ok = transfer[5].(float64)
	if !ok {
		return nil, fmt.Errorf("failed to typecast transfer field 5 to float64")
	}
	return ret, nil
}
