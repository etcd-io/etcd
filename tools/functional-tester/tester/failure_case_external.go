// Copyright 2018 The etcd Authors
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

package tester

import (
	"fmt"
	"os/exec"
)

type failureExternal struct {
	Failure

	description string
	scriptPath  string
}

func (f *failureExternal) Inject(clus *Cluster) error {
	return exec.Command(f.scriptPath, "enable", fmt.Sprintf("%d", clus.rd)).Run()
}

func (f *failureExternal) Recover(clus *Cluster) error {
	return exec.Command(f.scriptPath, "disable", fmt.Sprintf("%d", clus.rd)).Run()
}

func (f *failureExternal) Desc() string { return f.description }

func newFailureExternal(scriptPath string) Failure {
	return &failureExternal{
		description: fmt.Sprintf("external fault injector (script: %q)", scriptPath),
		scriptPath:  scriptPath,
	}
}
