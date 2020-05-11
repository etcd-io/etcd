// Copyright 2018 CoreOS, Inc.
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

package activation

import (
	"fmt"
	"os"
)

// exampleCmd returns the command line for the specified example binary.
func exampleCmd(binaryName string) (string, []string) {
	sourcePath := fmt.Sprintf("../examples/activation/%s.go", binaryName)
	sourceCmdLine := []string{"go", "run", sourcePath}
	binaryPath := fmt.Sprintf("../test_bins/%s.example", binaryName)
	if _, err := os.Stat(binaryPath); err != nil && os.IsNotExist(err) {
		return sourceCmdLine[0], sourceCmdLine[1:]
	}
	return binaryPath, []string{binaryPath}
}
