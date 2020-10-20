// Copyright 2016 The etcd Authors
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

// +build libs

// This file implements that pattern:
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// for etcd. Thanks to this file 'go mod tidy' does not removes dependencies.

package libs

import (
	_ "github.com/gogo/protobuf/proto"
)
