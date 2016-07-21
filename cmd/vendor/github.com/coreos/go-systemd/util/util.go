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

// Package util contains utility functions related to systemd that applications
// can use to check things like whether systemd is running.  Note that some of
// these functions attempt to manually load systemd libraries at runtime rather
// than linking against them.
package util

// #include <stdlib.h>
// #include <sys/types.h>
// #include <unistd.h>
//
// int
// my_sd_pid_get_owner_uid(void *f, pid_t pid, uid_t *uid)
// {
//   int (*sd_pid_get_owner_uid)(pid_t, uid_t *);
//
//   sd_pid_get_owner_uid = (int (*)(pid_t, uid_t *))f;
//   return sd_pid_get_owner_uid(pid, uid);
// }
//
// int
// my_sd_pid_get_unit(void *f, pid_t pid, char **unit)
// {
//   int (*sd_pid_get_unit)(pid_t, char **);
//
//   sd_pid_get_unit = (int (*)(pid_t, char **))f;
//   return sd_pid_get_unit(pid, unit);
// }
//
// int
// my_sd_pid_get_slice(void *f, pid_t pid, char **slice)
// {
//   int (*sd_pid_get_slice)(pid_t, char **);
//
//   sd_pid_get_slice = (int (*)(pid_t, char **))f;
//   return sd_pid_get_slice(pid, slice);
// }
//
// int
// am_session_leader()
// {
//   return (getsid(0) == getpid());
// }
import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var libsystemdNames = []string{
	// systemd < 209
	"libsystemd-login.so.0",
	"libsystemd-login.so",

	// systemd >= 209 merged libsystemd-login into libsystemd proper
	"libsystemd.so.0",
	"libsystemd.so",
}

// IsRunningSystemd checks whether the host was booted with systemd as its init
// system. This functions similarly to systemd's `sd_booted(3)`: internally, it
// checks whether /run/systemd/system/ exists and is a directory.
// http://www.freedesktop.org/software/systemd/man/sd_booted.html
func IsRunningSystemd() bool {
	fi, err := os.Lstat("/run/systemd/system")
	if err != nil {
		return false
	}
	return fi.IsDir()
}

// GetMachineID returns a host's 128-bit machine ID as a string. This functions
// similarly to systemd's `sd_id128_get_machine`: internally, it simply reads
// the contents of /etc/machine-id
// http://www.freedesktop.org/software/systemd/man/sd_id128_get_machine.html
func GetMachineID() (string, error) {
	machineID, err := ioutil.ReadFile("/etc/machine-id")
	if err != nil {
		return "", fmt.Errorf("failed to read /etc/machine-id: %v", err)
	}
	return strings.TrimSpace(string(machineID)), nil
}
