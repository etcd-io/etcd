// Copyright 2023 The etcd Authors
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

package watchdog

import "go.etcd.io/etcd/pkg/v3/watchdog"

var (
	storageWatchdog watchdog.Interface
)

func SetStorageWatchdog(wd watchdog.Interface) {
	storageWatchdog = wd
}

func StorageWatchdog() watchdog.Interface {
	if storageWatchdog != nil {
		return storageWatchdog
	}
	// Storage watchdog isn't enabled.
	return &NoopWatchdog{}
}

type NoopWatchdog struct{}

func (wd *NoopWatchdog) Register(name string) func() {
	return func() {}
}

func (wd *NoopWatchdog) Execute(name string, fn func()) {
	fn()
}
