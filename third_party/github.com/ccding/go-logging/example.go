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

package main

import (
	"github.com/ccding/go-logging/logging"
	"time"
)

func main() {
	logger1, _ := logging.SimpleLogger("logger1")
	logger1.SetLevel(logging.NOTSET)
	logger1.Error("this is a test from error")
	logger1.Debug("this is a test from debug")
	logger1.Notset("orz", time.Now().UnixNano())
	logger1.Destroy()

	logger2, _ := logging.RichLogger("logger2")
	logger2.SetLevel(logging.DEBUG)
	logger2.Error("this is a test from error")
	logger2.Debug("this is a test from debug")
	logger2.Notset("orz", time.Now().UnixNano())
	logger2.Destroy()

	logger3, _ := logging.ConfigLogger("example.conf")
	logger3.SetLevel(logging.DEBUG)
	logger3.Error("this is a test from error")
	logger3.Debug("this is a test from debug")
	logger3.Notset("orz", time.Now().UnixNano())
	logger3.Destroy()
}
