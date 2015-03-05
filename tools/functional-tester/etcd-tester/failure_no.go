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

package main

type failureBase struct {
	description
}

func newFailureBase() *failureBase {
	return &failureBase{
		description: "do nothing",
	}
}

func (f *failureBase) Inject(c *cluster) error { return nil }

func (f *failureBase) Recover(c *cluster) error { return nil }
