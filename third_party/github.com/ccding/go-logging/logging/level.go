// Copyright 2013, Cong Ding. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// author: Cong Ding <dinggnu@gmail.com>

package logging

// Level is the type of level.
type Level int32

// Values of level
const (
	CRITICAL Level = 50
	FATAL    Level = CRITICAL
	ERROR    Level = 40
	WARNING  Level = 30
	WARN     Level = WARNING
	INFO     Level = 20
	DEBUG    Level = 10
	NOTSET   Level = 0
)

// The mapping from level to level name
var levelNames = map[Level]string{
	CRITICAL: "CRITICAL",
	ERROR:    "ERROR",
	WARNING:  "WARNING",
	INFO:     "INFO",
	DEBUG:    "DEBUG",
	NOTSET:   "NOTSET",
}

// The mapping from level name to level
var levelValues = map[string]Level{
	"CRITICAL": CRITICAL,
	"ERROR":    ERROR,
	"WARN":     WARNING,
	"WARNING":  WARNING,
	"INFO":     INFO,
	"DEBUG":    DEBUG,
	"NOTSET":   NOTSET,
}

// String function casts level value to string
func (level *Level) String() string {
	return levelNames[*level]
}

// GetLevelName lets users be able to get level name from level value.
func GetLevelName(levelValue Level) string {
	return levelNames[levelValue]
}

// GetLevelValue lets users be able to get level value from level name.
func GetLevelValue(levelName string) Level {
	return levelValues[levelName]
}
