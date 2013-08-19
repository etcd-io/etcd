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

package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var commentPrefix = []string{"//", "#", ";"}

// Config struct constructs a new configuration handler.
type Config struct {
	filename string
	config   map[string]map[string]string
}

// NewConfig function cnstructs a new Config struct with filename. You have to
// call Read() function to let it read from the file. Otherwise you will get
// empty string (i.e., "") when you are calling Get() function. Another usage
// is that you call NewConfig() function and then call Add()/Set() function to
// add new key-values to the configuration. Finally you can call Write()
// function to write the new configuration to the file.
func NewConfig(filename string) *Config {
	c := new(Config)
	c.filename = filename
	c.config = make(map[string]map[string]string)
	return c
}

// Filename function returns the filename of the configuration.
func (c *Config) Filename() string {
	return c.filename
}

// SetFilename function sets the filename of the configuration.
func (c *Config) SetFilename(filename string) {
	c.filename = filename
}

// Reset function reset the map in the configuration.
func (c *Config) Reset() {
	c.config = make(map[string]map[string]string)
}

// Read function reads configurations from the file defined in
// Config.filename.
func (c *Config) Read() error {
	in, err := os.Open(c.filename)
	if err != nil {
		return err
	}
	defer in.Close()
	scanner := bufio.NewScanner(in)
	line := ""
	section := ""
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		if line == "" {
			sec, ok := checkSection(scanner.Text())
			if ok {
				section = sec
				continue
			}
		}
		if checkComment(scanner.Text()) {
			continue
		}
		line += scanner.Text()
		if strings.HasSuffix(line, "\\") {
			line = line[:len(line)-1]
			continue
		}
		key, value, ok := checkLine(line)
		if !ok {
			return errors.New("WRONG: " + line)
		}
		c.Set(section, key, value)
		line = ""
	}
	return nil
}

// Get function returns the value of a key in the configuration. If the key
// does not exist, it returns empty string (i.e., "").
func (c *Config) Get(section string, key string) string {
	value, ok := c.config[section][key]
	if !ok {
		return ""
	}
	return value
}

// Set function updates the value of a key in the configuration. Function
// Set() is exactly the same as function Add().
func (c *Config) Set(section string, key string, value string) {
	_, ok := c.config[section]
	if !ok {
		c.config[section] = make(map[string]string)
	}
	c.config[section][key] = value
}

// Add function adds a new key to the configuration. Function Add() is exactly
// the same as function Set().
func (c *Config) Add(section string, key string, value string) {
	c.Set(section, key, value)
}

// Del function deletes a key from the configuration.
func (c *Config) Del(section string, key string) {
	_, ok := c.config[section]
	if ok {
		delete(c.config[section], key)
		if len(c.config[section]) == 0 {
			delete(c.config, section)
		}
	}
}

// Write function writes the updated configuration back.
func (c *Config) Write() error {
	return nil
}

// WriteTo function writes the configuration to a new file. This function
// re-organizes the configuration and deletes all the comments.
func (c *Config) WriteTo(filename string) error {
	content := ""
	for k, v := range c.config {
		format := "%v = %v\n"
		if k != "" {
			content += fmt.Sprintf("[%v]\n", k)
			format = "\t" + format
		}
		for key, value := range v {
			content += fmt.Sprintf(format, key, value)
		}
	}
	return ioutil.WriteFile(filename, []byte(content), 0644)
}

// To check this line is a section or not. If it is not a section, it returns
// "".
func checkSection(line string) (string, bool) {
	line = strings.TrimSpace(line)
	lineLen := len(line)
	if lineLen < 2 {
		return "", false
	}
	if line[0] == '[' && line[lineLen-1] == ']' {
		return line[1 : lineLen-1], true
	}
	return "", false
}

// To check this line is a valid key-value pair or not.
func checkLine(line string) (string, string, bool) {
	key := ""
	value := ""
	sp := strings.SplitN(line, "=", 2)
	if len(sp) != 2 {
		return key, value, false
	}
	key = strings.TrimSpace(sp[0])
	value = strings.TrimSpace(sp[1])
	return key, value, true
}

// To check this line is a whole line comment or not.
func checkComment(line string) bool {
	line = strings.TrimSpace(line)
	for p := range commentPrefix {
		if strings.HasPrefix(line, commentPrefix[p]) {
			return true
		}
	}
	return false
}
