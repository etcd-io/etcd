// Copyright 2026 The etcd Authors
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

//go:build tools

// This file follows the tool-dependency tracking pattern:
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
//
// syft lives in its own module, deliberately outside go.work, because it
// pulls in a large dependency set (containerd, newer protobuf, etc.) that
// conflicts with the versions pinned across the etcd workspace. Keeping it
// here isolates those dependencies from the workspace 'verify-dep' check.

package tools

import (
	_ "github.com/anchore/syft/cmd/syft"
)
