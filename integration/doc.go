// Copyright 2014 CoreOS Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
functional tests are built upon embeded etcd, and focus on etcd functional
correctness.

Its goal:
1. it tests the whole code base except the command line parse.
2. it is able to check internal data, including raft, store and etc.
3. it is based on goroutine, which is faster than process.
4. it mainly tests user behavior and user-facing API.
*/
package integration
