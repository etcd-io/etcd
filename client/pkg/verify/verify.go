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

const envVerify = "ETCD_VERIFY"

type VerificationType string

const (
	envVerifyValueAll    VerificationType = "all"
	envVerifyValueAssert VerificationType = "assert"
)

func getEnvVerify() string {
	return strings.ToLower(os.Getenv(envVerify))
}

func IsVerificationEnabled(verification VerificationType) bool {
	env := getEnvVerify()
	return env == string(envVerifyValueAll) || env == strings.ToLower(string(verification))
}

// restoreEnv returns a function that restores envVerify to its current
// state, preserving whether it was set at all.
func restoreEnv() func() {
	previousEnv, wasSet := os.LookupEnv(envVerify)
	return func() {
		if wasSet {
			os.Setenv(envVerify, previousEnv)
		} else {
			os.Unsetenv(envVerify)
		}
	}
}

// EnableVerifications sets `envVerify` and returns a function that
// can be used to bring the original settings.
func EnableVerifications(verification VerificationType) func() {
	restore := restoreEnv()
	os.Setenv(envVerify, string(verification))
	return restore
}

// EnableAllVerifications enables verification and returns a function
// that can be used to bring the original settings.
func EnableAllVerifications() func() {
	return EnableVerifications(envVerifyValueAll)
}

// DisableVerifications unsets `envVerify` and returns a function that
// can be used to bring the original settings.
func DisableVerifications() func() {
	restore := restoreEnv()
	os.Unsetenv(envVerify)
	return restore
}

// Verify performs verification if the assertions are enabled.
// In the default setup running in tests and skipped in the production code.
func Verify(msg string, f VerifyFunc) {
	if IsVerificationEnabled(envVerifyValueAssert) {
		ok, details := f()
		verifier(ok, msg, details)
	}
}

type VerifyFunc func() (condition bool, details map[string]any)

func verifier(condition bool, msg string, details map[string]any) {
	if !condition {
		panic(fmt.Sprintf("%s. details: %v.", msg, details))
	}
}

// Assert will panic with a given formatted message if the given condition is false.
func Assert(condition bool, msg string, v ...any) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}
