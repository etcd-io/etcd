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

package memory

import (
	"github.com/shirou/gopsutil/mem"
)

// MemTotal returns the total amount of RAM on this system
func MemTotal() (uint64, error) {
	v, err := mem.VirtualMemory()
	return v.Total, err
}

// MemUsed returns the total used amount of RAM on this system
func MemUsed() (uint64, error) {
	v, err := mem.VirtualMemory()
	return v.Total - (v.Free + v.Buffers + v.Cached), err
}
