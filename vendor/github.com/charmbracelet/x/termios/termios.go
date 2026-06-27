//go:build darwin || netbsd || freebsd || openbsd || linux || dragonfly || solaris
// +build darwin netbsd freebsd openbsd linux dragonfly solaris

package termios

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// SetWinsize sets window size for an fd from a Winsize.
func SetWinsize(fd int, w *unix.Winsize) error {
	return unix.IoctlSetWinsize(fd, ioctlSetWinSize, w)
}

// GetWinsize gets window size for an fd.
func GetWinsize(fd int) (*unix.Winsize, error) {
	return unix.IoctlGetWinsize(fd, ioctlGetWinSize)
}

// GetTermios gets the termios of the given fd.
func GetTermios(fd int) (*unix.Termios, error) {
	return unix.IoctlGetTermios(fd, ioctlGets)
}

// SetTermios sets the given termios over the given fd's current termios.
func SetTermios(
	fd int,
	ispeed uint32,
	ospeed uint32,
	cc map[CC]uint8,
	iflag map[I]bool,
	oflag map[O]bool,
	cflag map[C]bool,
	lflag map[L]bool,
) error {
	term, err := unix.IoctlGetTermios(fd, ioctlGets)
	if err != nil {
		return err
	}
	setSpeed(term, ispeed, ospeed)

	for key, value := range cc {
		call, ok := allCcOpts[key]
		if !ok {
			continue
		}
		term.Cc[call] = value
	}

	for key, value := range iflag {
		mask, ok := allInputOpts[key]
		if ok {
			if value {
				term.Iflag |= bit(mask)
			} else {
				term.Iflag &= ^bit(mask)
			}
		}
	}
	for key, value := range oflag {
		mask, ok := allOutputOpts[key]
		if ok {
			if value {
				term.Oflag |= bit(mask)
			} else {
				term.Oflag &= ^bit(mask)
			}
		}
	}
	for key, value := range cflag {
		mask, ok := allControlOpts[key]
		if ok {
			if value {
				term.Cflag |= bit(mask)
			} else {
				term.Cflag &= ^bit(mask)
			}
		}
	}
	for key, value := range lflag {
		mask, ok := allLineOpts[key]
		if ok {
			if value {
				term.Lflag |= bit(mask)
			} else {
				term.Lflag &= ^bit(mask)
			}
		}
	}
	return unix.IoctlSetTermios(fd, ioctlSets, term)
}

// CC is the termios cc field.
//
// It stores an array of special characters related to terminal I/O.
type CC uint8

// CC possible values.
const (
	INTR CC = iota
	QUIT
	ERASE
	KILL
	EOF
	EOL
	EOL2
	START
	STOP
	SUSP
	WERASE
	RPRNT
	LNEXT
	DISCARD
	STATUS
	SWTCH
	DSUSP
	FLUSH
)

// https://www.man7.org/linux/man-pages/man3/termios.3.html
var allCcOpts = map[CC]int{
	INTR:    syscall.VINTR,
	QUIT:    syscall.VQUIT,
	ERASE:   syscall.VERASE,
	KILL:    syscall.VQUIT,
	EOF:     syscall.VEOF,
	EOL:     syscall.VEOL,
	EOL2:    syscall.VEOL2,
	START:   syscall.VSTART,
	STOP:    syscall.VSTOP,
	SUSP:    syscall.VSUSP,
	WERASE:  syscall.VWERASE,
	RPRNT:   syscall.VREPRINT,
	LNEXT:   syscall.VLNEXT,
	DISCARD: syscall.VDISCARD,

	// XXX: these syscalls don't exist for any OS
	// FLUSH:  syscall.VFLUSH,
}

// Input Controls
type I uint8

// Input possible values.
const (
	IGNPAR I = iota
	PARMRK
	INPCK
	ISTRIP
	INLCR
	IGNCR
	ICRNL
	IXON
	IXANY
	IXOFF
	IMAXBEL
	IUCLC
)

var allInputOpts = map[I]uint32{
	IGNPAR:  syscall.IGNPAR,
	PARMRK:  syscall.PARMRK,
	INPCK:   syscall.INPCK,
	ISTRIP:  syscall.ISTRIP,
	INLCR:   syscall.INLCR,
	IGNCR:   syscall.IGNCR,
	ICRNL:   syscall.ICRNL,
	IXON:    syscall.IXON,
	IXANY:   syscall.IXANY,
	IXOFF:   syscall.IXOFF,
	IMAXBEL: syscall.IMAXBEL,
}

// Output Controls
type O uint8

// Output possible values.
const (
	OPOST O = iota
	ONLCR
	OCRNL
	ONOCR
	ONLRET
	OLCUC
)

var allOutputOpts = map[O]uint32{
	OPOST:  syscall.OPOST,
	ONLCR:  syscall.ONLCR,
	OCRNL:  syscall.OCRNL,
	ONOCR:  syscall.ONOCR,
	ONLRET: syscall.ONLRET,
}

// Control
type C uint8

// Control possible values.
const (
	CS7 C = iota
	CS8
	PARENB
	PARODD
)

var allControlOpts = map[C]uint32{
	CS7:    syscall.CS7,
	CS8:    syscall.CS8,
	PARENB: syscall.PARENB,
	PARODD: syscall.PARODD,
}

// Line Controls.
type L uint8

// Line possible values.
const (
	ISIG L = iota
	ICANON
	ECHO
	ECHOE
	ECHOK
	ECHONL
	NOFLSH
	TOSTOP
	IEXTEN
	ECHOCTL
	ECHOKE
	PENDIN
	IUTF8
	XCASE
)

var allLineOpts = map[L]uint32{
	ISIG:    syscall.ISIG,
	ICANON:  syscall.ICANON,
	ECHO:    syscall.ECHO,
	ECHOE:   syscall.ECHOE,
	ECHOK:   syscall.ECHOK,
	ECHONL:  syscall.ECHONL,
	NOFLSH:  syscall.NOFLSH,
	TOSTOP:  syscall.TOSTOP,
	IEXTEN:  syscall.IEXTEN,
	ECHOCTL: syscall.ECHOCTL,
	ECHOKE:  syscall.ECHOKE,
	PENDIN:  syscall.PENDIN,
}
