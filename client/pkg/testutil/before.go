// Copyright 2022 The etcd Authors
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

package testutil

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// These are constants from "go.etcd.io/etcd/server/v3/verify",
// but we don't want to take dependency.
const ENV_VERIFY = "ETCD_VERIFY"
const ENV_VERIFY_ALL_VALUE = "all"

func BeforeTest(t testing.TB) {
	RegisterLeakDetection(t)
	os.Setenv(ENV_VERIFY, ENV_VERIFY_ALL_VALUE)

	path, err := os.Getwd()
	assert.NoError(t, err)
	tempDir := t.TempDir()
	assert.NoError(t, os.Chdir(tempDir))
	t.Logf("Changing working directory to: %s", tempDir)

	t.Cleanup(func() { assert.NoError(t, os.Chdir(path)) })
}

func BeforeIntegrationExamples(*testing.M) func() {
	ExitInShortMode("Skipping: the tests require real cluster")

	tempDir, err := ioutil.TempDir(os.TempDir(), "etcd-integration")
	if err != nil {
		log.Printf("Failed to obtain tempDir: %v", tempDir)
		os.Exit(1)
	}

	err = os.Chdir(tempDir)
	if err != nil {
		log.Printf("Failed to change working dir to: %s: %v", tempDir, err)
		os.Exit(1)
	}
	log.Printf("Running tests (examples) in dir(%v): ...", tempDir)
	return func() { os.RemoveAll(tempDir) }
}
