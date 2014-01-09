// Integration with the systemd D-Bus API.  See http://www.freedesktop.org/wiki/Software/systemd/dbus/
package dbus

import (
	"github.com/guelfey/go.dbus"
	"sync"
)

const signalBuffer = 100

type Conn struct {
	sysconn     *dbus.Conn
	sysobj      *dbus.Object
	jobListener struct {
		jobs map[dbus.ObjectPath]chan string
		sync.Mutex
	}
	subscriber struct {
		updateCh chan<- *SubStateUpdate
		errCh    chan<- error
		sync.Mutex
		ignore      map[dbus.ObjectPath]int64
		cleanIgnore int64
	}
	dispatch map[string]func(dbus.Signal)
}

func New() (*Conn, error) {
	c := new(Conn)

	if err := c.initConnection(); err != nil {
		return nil, err
	}

	c.initJobs()
	c.initSubscription()
	c.initDispatch()
	return c, nil
}

func (c *Conn) initConnection() error {
	var err error
	c.sysconn, err = dbus.SystemBusPrivate()
	if err != nil {
		return err
	}

	err = c.sysconn.Auth(nil)
	if err != nil {
		c.sysconn.Close()
		return err
	}

	err = c.sysconn.Hello()
	if err != nil {
		c.sysconn.Close()
		return err
	}

	c.sysobj = c.sysconn.Object("org.freedesktop.systemd1", dbus.ObjectPath("/org/freedesktop/systemd1"))

	c.sysconn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.systemd1.Manager',member='JobRemoved'")
	c.sysconn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.systemd1.Manager',member='UnitNew'")
	c.sysconn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged'")

	err = c.sysobj.Call("org.freedesktop.systemd1.Manager.Subscribe", 0).Store()
	if err != nil {
		c.sysconn.Close()
		return err
	}

	return nil
}

func (c *Conn) initDispatch() {
	ch := make(chan *dbus.Signal, signalBuffer)

	c.sysconn.Signal(ch)

	go func() {
		for {
			signal := <-ch
			switch signal.Name {
			case "org.freedesktop.systemd1.Manager.JobRemoved":
				c.jobComplete(signal)

				unitName := signal.Body[2].(string)
				var unitPath dbus.ObjectPath
				c.sysobj.Call("GetUnit", 0, unitName).Store(&unitPath)
				if unitPath != dbus.ObjectPath("") {
					c.sendSubStateUpdate(unitPath)
				}
			case "org.freedesktop.systemd1.Manager.UnitNew":
				c.sendSubStateUpdate(signal.Body[1].(dbus.ObjectPath))
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				if signal.Body[0].(string) == "org.freedesktop.systemd1.Unit" {
					// we only care about SubState updates, which are a Unit property
					c.sendSubStateUpdate(signal.Path)
				}
			}
		}
	}()
}
