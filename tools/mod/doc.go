// Copyright 2024 The etcd Authors
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

// As this directory implements the pattern for tracking tool dependencies as documented here:
// https://go.dev/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module, it doesn't
// contain any valid go source code in the directory directly. This would break scripts for
// unit testing, golangci-lint, and coverage calculation.
//
// Thus, to ensure tools to run normally, we've added this empty file.

package mod
