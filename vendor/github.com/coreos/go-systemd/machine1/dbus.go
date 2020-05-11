/*
Copyright 2015 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Integration with the systemd machined API.  See http://www.freedesktop.org/wiki/Software/systemd/machined/
package machine1

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/godbus/dbus"

	sd_dbus "github.com/coreos/go-systemd/dbus"
)

const (
	dbusInterface = "org.freedesktop.machine1.Manager"
	dbusPath      = "/org/freedesktop/machine1"
)

// Conn is a connection to systemds dbus endpoint.
type Conn struct {
	conn   *dbus.Conn
	object dbus.BusObject
}

// MachineStatus is a set of necessary info for each machine
type MachineStatus struct {
	Name    string          // The primary machine name as string
	Class   string          // The machine class as string
	Service string          // The machine service as string
	JobPath dbus.ObjectPath // The job object path
}

// ImageStatus is a set of necessary info for each machine image
type ImageStatus struct {
	Name       string          // The primary image name as string
	ImageType  string          // The image type as string
	Readonly   bool            // whether it's readonly or not
	CreateTime uint64          // time when it's created
	ModifyTime uint64          // time when it's modified
	DiskUsage  uint64          // used disk space
	JobPath    dbus.ObjectPath // The job object path
}

// New() establishes a connection to the system bus and authenticates.
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

	c.object = c.conn.Object("org.freedesktop.machine1", dbus.ObjectPath(dbusPath))

	return nil
}

func (c *Conn) getPath(method string, args ...interface{}) (dbus.ObjectPath, error) {
	result := c.object.Call(fmt.Sprintf("%s.%s", dbusInterface, method), 0, args...)
	if result.Err != nil {
		return "", result.Err
	}

	path, typeErr := result.Body[0].(dbus.ObjectPath)
	if !typeErr {
		return "", fmt.Errorf("unable to convert dbus response '%v' to dbus.ObjectPath", result.Body[0])
	}

	return path, nil
}

// CreateMachine creates a new virtual machine or container with systemd-machined, generating a scope unit for it
func (c *Conn) CreateMachine(name string, id []byte, service string, class string, pid int, root_directory string, scope_properties []sd_dbus.Property) error {
	return c.object.Call(dbusInterface+".CreateMachine", 0, name, id, service, class, uint32(pid), root_directory, scope_properties).Err
}

// CreateMachineWithNetwork creates the container with its network config with systemd-machined
func (c *Conn) CreateMachineWithNetwork(name string, id []byte, service string, class string, pid int, root_directory string, ifindices []int, scope_properties []sd_dbus.Property) error {
	return c.object.Call(dbusInterface+".CreateMachineWithNetwork", 0, name, id, service, class, uint32(pid), root_directory, ifindices, scope_properties).Err
}

// GetMachine gets a specific container with systemd-machined
func (c *Conn) GetMachine(name string) (dbus.ObjectPath, error) {
	return c.getPath("GetMachine", name)
}

// GetImage gets a specific image with systemd-machined
func (c *Conn) GetImage(name string) (dbus.ObjectPath, error) {
	return c.getPath("GetImage", name)
}

// GetMachineByPID gets a machine specified by a PID from systemd-machined
func (c *Conn) GetMachineByPID(pid uint) (dbus.ObjectPath, error) {
	return c.getPath("GetMachineByPID", pid)
}

// GetMachineAddresses gets a list of IP addresses
func (c *Conn) GetMachineAddresses(name string) (dbus.ObjectPath, error) {
	return c.getPath("GetMachineAddresses", name)
}

// DescribeMachine gets the properties of a machine
func (c *Conn) DescribeMachine(name string) (machineProps map[string]interface{}, err error) {
	var dbusProps map[string]dbus.Variant
	path, pathErr := c.GetMachine(name)
	if pathErr != nil {
		return nil, pathErr
	}
	obj := c.conn.Object("org.freedesktop.machine1", path)
	err = obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, "").Store(&dbusProps)
	if err != nil {
		return nil, err
	}
	machineProps = make(map[string]interface{}, len(dbusProps))
	for key, val := range dbusProps {
		machineProps[key] = val.Value()
	}
	return
}

// KillMachine sends a signal to a machine
func (c *Conn) KillMachine(name, who string, sig syscall.Signal) error {
	return c.object.Call(dbusInterface+".KillMachine", 0, name, who, sig).Err
}

// TerminateMachine causes systemd-machined to terminate a machine, killing its processes
func (c *Conn) TerminateMachine(name string) error {
	return c.object.Call(dbusInterface+".TerminateMachine", 0, name).Err
}

// RegisterMachine registers the container with the systemd-machined
func (c *Conn) RegisterMachine(name string, id []byte, service string, class string, pid int, root_directory string) error {
	return c.object.Call(dbusInterface+".RegisterMachine", 0, name, id, service, class, uint32(pid), root_directory).Err
}

// RegisterMachineWithNetwork registers the container with its network with systemd-machined
func (c *Conn) RegisterMachineWithNetwork(name string, id []byte, service string, class string, pid int, root_directory string, ifindices []int) error {
	return c.object.Call(dbusInterface+".RegisterMachineWithNetwork", 0, name, id, service, class, uint32(pid), root_directory, ifindices).Err
}

func machineFromInterfaces(machine []interface{}) (*MachineStatus, error) {
	if len(machine) < 4 {
		return nil, fmt.Errorf("invalid number of machine fields: %d", len(machine))
	}
	name, ok := machine[0].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast machine field 0 to string")
	}
	class, ok := machine[1].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast class field 1 to string")
	}
	service, ok := machine[2].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast service field 2 to string")
	}
	jobpath, ok := machine[3].(dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("failed to typecast jobpath field 3 to ObjectPath")
	}

	ret := MachineStatus{Name: name, Class: class, Service: service, JobPath: jobpath}
	return &ret, nil
}

// ListMachines returns an array of all currently running machines.
func (c *Conn) ListMachines() ([]MachineStatus, error) {
	result := make([][]interface{}, 0)
	if err := c.object.Call(dbusInterface+".ListMachines", 0).Store(&result); err != nil {
		return nil, err
	}

	machs := []MachineStatus{}
	for _, i := range result {
		machine, err := machineFromInterfaces(i)
		if err != nil {
			return nil, err
		}
		machs = append(machs, *machine)
	}

	return machs, nil
}

func imageFromInterfaces(image []interface{}) (*ImageStatus, error) {
	if len(image) < 7 {
		return nil, fmt.Errorf("invalid number of image fields: %d", len(image))
	}
	name, ok := image[0].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast image field 0 to string")
	}
	imagetype, ok := image[1].(string)
	if !ok {
		return nil, fmt.Errorf("failed to typecast imagetype field 1 to string")
	}
	readonly, ok := image[2].(bool)
	if !ok {
		return nil, fmt.Errorf("failed to typecast readonly field 2 to bool")
	}
	createtime, ok := image[3].(uint64)
	if !ok {
		return nil, fmt.Errorf("failed to typecast createtime field 3 to uint64")
	}
	modifytime, ok := image[4].(uint64)
	if !ok {
		return nil, fmt.Errorf("failed to typecast modifytime field 4 to uint64")
	}
	diskusage, ok := image[5].(uint64)
	if !ok {
		return nil, fmt.Errorf("failed to typecast diskusage field 5 to uint64")
	}
	jobpath, ok := image[6].(dbus.ObjectPath)
	if !ok {
		return nil, fmt.Errorf("failed to typecast jobpath field 6 to ObjectPath")
	}

	ret := ImageStatus{Name: name, ImageType: imagetype, Readonly: readonly, CreateTime: createtime, ModifyTime: modifytime, DiskUsage: diskusage, JobPath: jobpath}
	return &ret, nil
}

// ListImages returns an array of all currently available images.
func (c *Conn) ListImages() ([]ImageStatus, error) {
	result := make([][]interface{}, 0)
	if err := c.object.Call(dbusInterface+".ListImages", 0).Store(&result); err != nil {
		return nil, err
	}

	images := []ImageStatus{}
	for _, i := range result {
		image, err := imageFromInterfaces(i)
		if err != nil {
			return nil, err
		}
		images = append(images, *image)
	}

	return images, nil
}
