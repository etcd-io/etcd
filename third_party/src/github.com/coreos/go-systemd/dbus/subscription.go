package dbus

import (
	"errors"
	"time"

	"github.com/guelfey/go.dbus"
)

const (
	cleanIgnoreInterval = int64(10 * time.Second)
	ignoreInterval      = int64(30 * time.Millisecond)
)

func (c *Conn) initSubscription() {
	c.subscriber.ignore = make(map[dbus.ObjectPath]int64)
}

// Returns two unbuffered channels which will receive all changed units every
// @interval@ seconds.  Deleted units are sent as nil.
func (c *Conn) SubscribeUnits(interval time.Duration) (<-chan map[string]*UnitStatus, <-chan error) {
	return c.SubscribeUnitsCustom(interval, 0, func(u1, u2 *UnitStatus) bool { return *u1 != *u2 })
}

// SubscribeUnitsCustom is like SubscribeUnits but lets you specify the buffer
// size of the channels and the comparison function for detecting changes.
func (c *Conn) SubscribeUnitsCustom(interval time.Duration, buffer int, isChanged func(*UnitStatus, *UnitStatus) bool) (<-chan map[string]*UnitStatus, <-chan error) {
	old := make(map[string]*UnitStatus)
	statusChan := make(chan map[string]*UnitStatus, buffer)
	errChan := make(chan error, buffer)

	go func() {
		for {
			timerChan := time.After(interval)

			units, err := c.ListUnits()
			if err == nil {
				cur := make(map[string]*UnitStatus)
				for i := range units {
					cur[units[i].Name] = &units[i]
				}

				// add all new or changed units
				changed := make(map[string]*UnitStatus)
				for n, u := range cur {
					if oldU, ok := old[n]; !ok || isChanged(oldU, u) {
						changed[n] = u
					}
					delete(old, n)
				}

				// add all deleted units
				for oldN := range old {
					changed[oldN] = nil
				}

				old = cur

				statusChan <- changed
			} else {
				errChan <- err
			}

			<-timerChan
		}
	}()

	return statusChan, errChan
}

type SubStateUpdate struct {
	UnitName string
	SubState string
}

// SetSubStateSubscriber writes to updateCh when any unit's substate changes.
// Althrough this writes to updateCh on every state change, the reported state
// may be more recent than the change that generated it (due to an unavoidable
// race in the systemd dbus interface).  That is, this method provides a good
// way to keep a current view of all units' states, but is not guaranteed to
// show every state transition they go through.  Furthermore, state changes
// will only be written to the channel with non-blocking writes.  If updateCh
// is full, it attempts to write an error to errCh; if errCh is full, the error
// passes silently.
func (c *Conn) SetSubStateSubscriber(updateCh chan<- *SubStateUpdate, errCh chan<- error) {
	c.subscriber.Lock()
	defer c.subscriber.Unlock()
	c.subscriber.updateCh = updateCh
	c.subscriber.errCh = errCh
}

func (c *Conn) sendSubStateUpdate(path dbus.ObjectPath) {
	c.subscriber.Lock()
	defer c.subscriber.Unlock()
	if c.subscriber.updateCh == nil {
		return
	}

	if c.shouldIgnore(path) {
		return
	}

	info, err := c.GetUnitInfo(path)
	if err != nil {
		select {
		case c.subscriber.errCh <- err:
		default:
		}
	}

	name := info["Id"].Value().(string)
	substate := info["SubState"].Value().(string)

	update := &SubStateUpdate{name, substate}
	select {
	case c.subscriber.updateCh <- update:
	default:
		select {
		case c.subscriber.errCh <- errors.New("update channel full!"):
		default:
		}
	}

	c.updateIgnore(path, info)
}

func (c *Conn) GetUnitInfo(path dbus.ObjectPath) (map[string]dbus.Variant, error) {
	var err error
	var props map[string]dbus.Variant
	obj := c.sysconn.Object("org.freedesktop.systemd1", path)
	err = obj.Call("GetAll", 0, "org.freedesktop.systemd1.Unit").Store(&props)
	if err != nil {
		return nil, err
	}
	return props, nil
}

// The ignore functions work around a wart in the systemd dbus interface.
// Requesting the properties of an unloaded unit will cause systemd to send a
// pair of UnitNew/UnitRemoved signals.  Because we need to get a unit's
// properties on UnitNew (as that's the only indication of a new unit coming up
// for the first time), we would enter an infinite loop if we did not attempt
// to detect and ignore these spurious signals.  The signal themselves are
// indistinguishable from relevant ones, so we (somewhat hackishly) ignore an
// unloaded unit's signals for a short time after requesting its properties.
// This means that we will miss e.g. a transient unit being restarted
// *immediately* upon failure and also a transient unit being started
// immediately after requesting its status (with systemctl status, for example,
// because this causes a UnitNew signal to be sent which then causes us to fetch
// the properties).

func (c *Conn) shouldIgnore(path dbus.ObjectPath) bool {
	t, ok := c.subscriber.ignore[path]
	return ok && t >= time.Now().UnixNano()
}

func (c *Conn) updateIgnore(path dbus.ObjectPath, info map[string]dbus.Variant) {
	c.cleanIgnore()

	// unit is unloaded - it will trigger bad systemd dbus behavior
	if info["LoadState"].Value().(string) == "not-found" {
		c.subscriber.ignore[path] = time.Now().UnixNano() + ignoreInterval
	}
}

// without this, ignore would grow unboundedly over time
func (c *Conn) cleanIgnore() {
	now := time.Now().UnixNano()
	if c.subscriber.cleanIgnore < now {
		c.subscriber.cleanIgnore = now + cleanIgnoreInterval

		for p, t := range c.subscriber.ignore {
			if t < now {
				delete(c.subscriber.ignore, p)
			}
		}
	}
}
