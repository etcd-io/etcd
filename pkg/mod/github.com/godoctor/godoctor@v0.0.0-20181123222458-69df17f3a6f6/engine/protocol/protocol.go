// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package protocol provides an implementation of the OpenRefactory protocol
// (server-side), which provides a standard mechanism for text editors to
// communicate with refactoring engines.
package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/godoctor/godoctor/filesystem"
)

type Reply struct {
	Params map[string]interface{}
}

func (r Reply) String() string {
	replyJson, _ := json.Marshal(r.Params)
	return string(replyJson)
}

type State struct {
	State      int
	About      string
	Mode       string
	Dir        string
	Filesystem filesystem.FileSystem
}

func Run(writer io.Writer, aboutText string, args []string) {

	// single command console
	if len(args) == 0 {
		runSingle(writer, aboutText)
		return
	}

	// list of commands
	var argJson []map[string]interface{}
	if len(args) == 1 && args[0] == "-" {
		// read command list from stdin
		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}})
			return
		}
		err = json.Unmarshal(bytes, &argJson)
		if err != nil {
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}})
			return
		}
	} else {
		// assemble command list from args
		err := json.Unmarshal([]byte(args[0]), &argJson)
		if err != nil {
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}})
			return
		}
	}
	runList(writer, aboutText, argJson)
}

func runSingle(writer io.Writer, aboutText string) {
	cmdList := setup()
	var state = State{State: 0, About: aboutText, Mode: "", Dir: "", Filesystem: nil}
	var inputJson map[string]interface{}
	ioreader := bufio.NewReader(os.Stdin)
	for {
		input, err := ioreader.ReadBytes('\n')
		if err == io.EOF {
			// exit
			break
		} else if err != nil {
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}})
			continue
		}
		err = json.Unmarshal(input, &inputJson)
		if err != nil {
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}})
			continue
		}
		// check command key exists
		cmd, found := inputJson["command"]
		if !found {
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": "Invalid JSON command"}})
			continue
		}
		// if close command, just exit
		if cmd == "close" {
			break
		}
		// check command is one we support
		if _, found := cmdList[cmd.(string)]; !found {
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": "Invalid JSON command"}})
			continue
		}
		// everything good to run command
		result, _ := cmdList[cmd.(string)](&state, inputJson) // run the command
		printReply(writer, result)
	}
}

func runList(writer io.Writer, aboutText string, argJson []map[string]interface{}) {
	cmdList := setup()
	var state = State{State: 1, About: aboutText, Mode: "", Dir: "", Filesystem: nil}
	for i, cmdObj := range argJson {
		// has command?
		cmd, found := cmdObj["command"]
		if !found { // no command
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": "Invalid JSON command"}})
			return
		}
		// valid command?
		if _, found := cmdList[cmd.(string)]; found {
			resultReply, err := cmdList[cmd.(string)](&state, cmdObj)
			if err != nil {
				printReply(writer, resultReply)
				return
			}
			// last command?
			if i == len(argJson)-1 {
				printReply(writer, resultReply)
			}
		} else {
			printReply(writer, Reply{map[string]interface{}{"reply": "Error", "message": "Invalid JSON command"}})
			return
		}
	}
}

// little helpers
func setup() map[string]Command {
	cmds := make(map[string]Command)
	cmds["about"] = about
	cmds["open"] = open
	cmds["list"] = list
	cmds["setdir"] = setdir
	cmds["params"] = params
	cmds["put"] = put
	cmds["xrun"] = xRun
	return cmds
}

func printReply(writer io.Writer, reply Reply) {
	fmt.Fprintf(writer, "%s\n", reply)
}
