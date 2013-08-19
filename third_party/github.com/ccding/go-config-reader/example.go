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
	"fmt"
	"github.com/ccding/go-config-reader/config"
)

func main() {
	c := config.NewConfig("example.conf")
	err := c.Read()
	fmt.Println(err)
	fmt.Println(c)
	fmt.Println(c.Get("test", "a"))
	fmt.Println(c.Get("", "dd"))
	c.WriteTo("example2.conf")
}
