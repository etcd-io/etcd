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
//
package logging

import (
	"fmt"
	"strconv"
	"testing"
)

func empty() {
}

func TestGetGoID(t *testing.T) {
	for i := 0; i < 1000; i++ {
		goid := int(GetGoID())
		go empty()
		goid2 := int(GetGoID())
		if goid != goid2 {
			t.Errorf("%v, %v\n", goid, goid2)
		}
	}
}

func TestSeqid(t *testing.T) {
	logger, _ := BasicLogger("test")
	for i := 0; i < 1000; i++ {
		r := new(record)
		name := strconv.Itoa(i + 1)
		seq := logger.nextSeqid(r)
		if fmt.Sprintf("%d", seq) != name {
			t.Errorf("%v, %v\n", seq, name)
		}
	}
	logger.Destroy()
}

func TestName(t *testing.T) {
	name := "test"
	logger, _ := BasicLogger(name)
	r := new(record)
	if logger.lname(r) != name {
		t.Errorf("%v, %v\n", logger.lname(r), name)
	}
	logger.Destroy()
}
