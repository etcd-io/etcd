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

package verify

import (
	"fmt"
	"os"
	"strings"
)

const ENV_VERIFY = "ETCD_VERIFY"

type VerificationType string

const (
	ENV_VERIFY_VALUE_ALL    VerificationType = "all"
	ENV_VERIFY_VALUE_ASSERT VerificationType = "assert"
)

func getEnvVerify() string {
	return strings.ToLower(os.Getenv(ENV_VERIFY))
}

func IsVerificationEnabled(verification VerificationType) bool {
	env := getEnvVerify()
	return env == string(ENV_VERIFY_VALUE_ALL) || env == strings.ToLower(string(verification))
}

// EnableVerifications sets `ENV_VERIFY` and returns a function that
// can be used to bring the original settings.
func EnableVerifications(verification VerificationType) func() {
	previousEnv := getEnvVerify()
	os.Setenv(ENV_VERIFY, string(verification))
	return func() {
		os.Setenv(ENV_VERIFY, previousEnv)
	}
}

// EnableAllVerifications enables verification and returns a function
// that can be used to bring the original settings.
func EnableAllVerifications() func() {
	return EnableVerifications(ENV_VERIFY_VALUE_ALL)
}

// DisableVerifications unsets `ENV_VERIFY` and returns a function that
// can be used to bring the original settings.
func DisableVerifications() func() {
	previousEnv := getEnvVerify()
	os.Unsetenv(ENV_VERIFY)
	return func() {
		os.Setenv(ENV_VERIFY, previousEnv)
	}
}

// Verify performs verification if the assertions are enabled.
// In the default setup running in tests and skipped in the production code.
func Verify(f func()) {
	if IsVerificationEnabled(ENV_VERIFY_VALUE_ASSERT) {
		f()
	}
}

// Assert will panic with a given formatted message if the given condition is false.
func Assert(condition bool, msg string, v ...any) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}
