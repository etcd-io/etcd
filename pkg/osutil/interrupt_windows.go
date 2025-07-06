// Copyright 2015 The etcd Authors
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

//go:build windows
// +build windows

package osutil

import (
	"os"

	"go.uber.org/zap"
)

type InterruptHandler func()

// RegisterInterruptHandler is a no-op on windows
func RegisterInterruptHandler(h InterruptHandler) {}

// HandleInterrupts is a no-op on windows
func HandleInterrupts(*zap.Logger) {}

// Exit calls os.Exit
func Exit(code int) {
	os.Exit(code)
}
