// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package timeutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

// init initializes `locCache`.
func init() {
	// We need set systemTZ when it is in testing process.
	if systemTZ == "" {
		systemTZ = "System"
	}
	locCa = &locCache{}
	locCa.locMap = make(map[string]*time.Location)
}

// locCa is a simple cache policy to improve the performance of 'time.LoadLocation'.
var locCa *locCache

// systemTZ is current TiDB's system timezone name.
var systemTZ string
var zoneSources = []string{
	"/usr/share/zoneinfo/",
	"/usr/share/lib/zoneinfo/",
	"/usr/lib/locale/TZ/",
	// this is for macOS
	"/var/db/timezone/zoneinfo/",
}

// locCache is a simple map with lock. It stores all used timezone during the lifetime of tidb instance.
// Talked with Golang team about whether they can have some forms of cache policy available for programmer,
// they suggests that only programmers knows which one is best for their use case.
// For detail, please refer to: https://github.com/golang/go/issues/26106
type locCache struct {
	sync.RWMutex
	// locMap stores locations used in past and can be retrieved by a timezone's name.
	locMap map[string]*time.Location
}

// InferSystemTZ reads system timezone from `TZ`, the path of the soft link of `/etc/localtime`. If both of them are failed, system timezone will be set to `UTC`.
// It is exported because we need to use it during bootstap stage. And it should be only used at that stage.
func InferSystemTZ() string {
	// consult $TZ to find the time zone to use.
	// no $TZ means use the system default /etc/localtime.
	// $TZ="" means use UTC.
	// $TZ="foo" means use /usr/share/zoneinfo/foo.
	tz, ok := syscall.Getenv("TZ")
	switch {
	case !ok:
		path, err1 := filepath.EvalSymlinks("/etc/localtime")
		if err1 == nil {
			name, err2 := inferTZNameFromFileName(path)
			if err2 == nil {
				return name
			}
			log.Errorln(err2)
		}
		log.Errorln(err1)
	case tz != "" && tz != "UTC":
		for _, source := range zoneSources {
			if _, err := os.Stat(source + tz); err == nil {
				return tz
			}
		}
	}
	return "UTC"
}

// inferTZNameFromFileName gets IANA timezone name from zoneinfo path.
// TODO: It will be refined later. This is just a quick fix.
func inferTZNameFromFileName(path string) (string, error) {
	// phase1 only support read /etc/localtime which is a softlink to zoneinfo file
	substr := "zoneinfo"
	// macOs MoJave changes the sofe link of /etc/localtime from
	// "/var/db/timezone/tz/2018e.1.0/zoneinfo/Asia/Shanghai"
	// to "/usr/share/zoneinfo.default/Asia/Shanghai"
	substrMojave := "zoneinfo.default"

	if idx := strings.Index(path, substrMojave); idx != -1 {
		return string(path[idx+len(substrMojave)+1:]), nil
	}

	if idx := strings.Index(path, substr); idx != -1 {
		return string(path[idx+len(substr)+1:]), nil
	}
	return "", errors.New(fmt.Sprintf("path %s is not supported", path))
}

// SystemLocation returns time.SystemLocation's IANA timezone location. It is TiDB's global timezone location.
func SystemLocation() *time.Location {
	loc, err := LoadLocation(systemTZ)
	if err != nil {
		return time.Local
	}
	return loc
}

var setSysTZOnce sync.Once

// SetSystemTZ sets systemTZ by the value loaded from mysql.tidb.
func SetSystemTZ(name string) {
	setSysTZOnce.Do(func() {
		systemTZ = name
	})
}

// getLoc first trying to load location from a cache map. If nothing found in such map, then call
// `time.LoadLocation` to get a timezone location. After trying both way, an error will be returned
//  if valid Location is not found.
func (lm *locCache) getLoc(name string) (*time.Location, error) {
	if name == "System" {
		return time.Local, nil
	}
	lm.RLock()
	v, ok := lm.locMap[name]
	lm.RUnlock()
	if ok {
		return v, nil
	}

	if loc, err := time.LoadLocation(name); err == nil {
		// assign value back to map
		lm.Lock()
		lm.locMap[name] = loc
		lm.Unlock()
		return loc, nil
	}

	return nil, fmt.Errorf("invalid name for timezone %s", name)
}

// LoadLocation loads time.Location by IANA timezone time.
func LoadLocation(name string) (*time.Location, error) {
	return locCa.getLoc(name)
}

// Zone returns the current timezone name and timezone offset in seconds.
// In compatible with MySQL, we change `SystemLocation` to `System`.
func Zone(loc *time.Location) (string, int64) {
	_, offset := time.Now().In(loc).Zone()
	var name string
	name = loc.String()
	// when we found name is "System", we have no chice but push down
	// "System" to tikv side.
	if name == "Local" {
		name = "System"
	}

	return name, int64(offset)
}
