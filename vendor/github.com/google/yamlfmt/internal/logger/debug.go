// Copyright 2024 Google LLC
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

package logger

import (
	"fmt"

	"github.com/google/yamlfmt/internal/collections"
)

type DebugCode int

const (
	DebugCodeAny DebugCode = iota
	DebugCodeConfig
	DebugCodePaths
	DebugCodeDiffs
)

var (
	supportedDebugCodes = map[string][]DebugCode{
		"config": {DebugCodeConfig},
		"paths":  {DebugCodePaths},
		"diffs":  {DebugCodeDiffs},
		"all":    {DebugCodeConfig, DebugCodePaths, DebugCodeDiffs},
	}
	activeDebugCodes = collections.Set[DebugCode]{}
)

func ActivateDebugCode(code string) {
	if debugCodes, ok := supportedDebugCodes[code]; ok {
		activeDebugCodes.Add(debugCodes...)
	}
}

func DebugCodeIsActive(code DebugCode) bool {
	return activeDebugCodes.Contains(code)
}

// Debug prints a message if the given debug code is active.
func Debug(code DebugCode, msg string, args ...any) {
	if activeDebugCodes.Contains(code) {
		fmt.Printf("[DEBUG]: %s\n", fmt.Sprintf(msg, args...))
	}
}
