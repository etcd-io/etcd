// +build linux darwin freebsd netbsd openbsd

package pb

import "syscall"

const sys_ioctl = syscall.SYS_IOCTL
