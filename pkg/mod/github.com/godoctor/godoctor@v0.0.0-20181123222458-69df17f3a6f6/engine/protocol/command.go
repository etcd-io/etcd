// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/filesystem"
	"github.com/godoctor/godoctor/refactoring"
	"github.com/godoctor/godoctor/text"
)

type Command func(*State, map[string]interface{}) (Reply, error)

// -=-= About =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=

func about(state *State, input map[string]interface{}) (Reply, error) {
	if err := aboutValidate(state, input); err != nil {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}
	return Reply{map[string]interface{}{"reply": "OK", "text": state.About}}, nil
}

func aboutValidate(state *State, input map[string]interface{}) error {
	if state.State > 0 {
		return nil
	} else {
		return errors.New("The about command requires a state of non-zero")
	}
}

// -=-= List =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// TODO add in implementation of fileselection and textselection keys

var listQualityChk = "in_testing|in_development|production"

func list(state *State, input map[string]interface{}) (Reply, error) {
	if err := listValidate(state, input); err != nil {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}
	hiddenOK := true
	switch input["quality"].(string) {
	case "in_testing":
		hiddenOK = false
	case "production":
		hiddenOK = false
	}

	// get all of the refactoring names
	namesList := make([]map[string]string, 0)
	for _, shortName := range engine.AllRefactoringNames() {
		refactoring := engine.GetRefactoring(shortName)
		if hiddenOK || !refactoring.Description().Hidden {
			namesList = append(namesList, map[string]string{"shortName": shortName, "name": refactoring.Description().Name})
		}
	}
	return Reply{map[string]interface{}{"reply": "OK", "transformations": namesList}}, nil
}

func listValidate(state *State, input map[string]interface{}) error {
	if state.State < 1 {
		err := errors.New("The about command requires a state of non-zero")
		return err
	}
	// check for required keys
	if _, found := input["quality"]; !found {
		err := errors.New("Quality key not found")
		return err
	}

	// check quality matches
	qualityValidator := regexp.MustCompile(listQualityChk)

	if valid := qualityValidator.MatchString(input["quality"].(string)); !valid {
		return errors.New("Quality key must be \"in_testing|in_development|production\"")
	}

	// validate text/file selection
	// TODO validate fileselection
	textselection, tsfound := input["textselection"]
	_, fsfound := input["fileselection"]

	if tsfound && fsfound {
		return errors.New("Both textseleciton and fileselection cannot be used together")
	} else if tsfound {
		if state.State < 2 {
			return errors.New("File system not yet configured, cannot use textselection")
		}
		_, err := parseSelection(state, textselection.(map[string]interface{}))
		if err != nil {
			return err
		}
	} else if fsfound {
		if state.State < 2 {
			return errors.New("File system not yet configured, cannot use fileselection")
		}
	}
	return nil
}

// -=-= Open =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// TODO open with version

func open(state *State, input map[string]interface{}) (Reply, error) {
	state.State = 1
	return Reply{map[string]interface{}{"reply": "OK"}}, nil
}

// basically useless until we implement versioning...
func openValidate(state *State, input map[string]interface{}) error {
	return nil
}

// -=-= Params =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

func params(state *State, input map[string]interface{}) (Reply, error) {
	//refactoring := engine.GetRefactoring("rename")
	if err := paramsValidate(state, input); err != nil {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}
	refactoring := engine.GetRefactoring(input["transformation"].(string))
	// since GetParams returns just a string, assume it as prompt and label
	params := make([]map[string]interface{}, 0)
	for _, param := range refactoring.Description().Params {
		params = append(params, map[string]interface{}{"label": param.Label, "prompt": param.Prompt, "type": reflect.TypeOf(param.DefaultValue).String(), "default": param.DefaultValue})
	}
	return Reply{map[string]interface{}{"reply": "OK", "params": params}}, nil
}

func paramsValidate(state *State, input map[string]interface{}) error {
	if state.State < 2 {
		return errors.New("State of 2 (file system configured) is required")
	}
	if _, found := input["transformation"]; !found {
		return errors.New("Transformation key not found")
	}
	// validate text/file selection
	// TODO validate fileselection
	textselection, tsfound := input["textselection"]
	_, fsfound := input["fileselection"]

	if tsfound && fsfound {
		return errors.New("Both textseleciton and fileselection cannot be used together")
	} else if tsfound {
		_, err := parseSelection(state, textselection.(map[string]interface{}))
		if err != nil {
			return err
		}
	} else if fsfound {

	}
	return nil
}

// -=-= Put -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

func put(state *State, input map[string]interface{}) (Reply, error) {
	if err := putValidate(state, input); err != nil {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}

	var editedFS *filesystem.EditedFileSystem
	editedFS = state.Filesystem.(*filesystem.EditedFileSystem)

	stdinPath, err := filesystem.FakeStdinPath()
	if err != nil {
		return Reply{map[string]interface{}{"reply": "Error",
			"message": err.Error()}}, err
	}

	es := text.NewEditSet()
	es.Add(&text.Extent{0, 0}, input["content"].(string))
	editedFS.Edits[stdinPath] = es
	return Reply{map[string]interface{}{"reply": "OK"}}, nil
}

func putValidate(state *State, input map[string]interface{}) error {
	// validate state
	if state.State < 2 {
		return fmt.Errorf("put requires state of 2 (file system configured)")
	}
	if state.Mode != "web" {
		return fmt.Errorf("put can only be executed in Web mode")
	}

	// validate input
	if _, found := input["filename"]; !found {
		return fmt.Errorf("filename is required")
	}
	if _, found := input["content"]; !found {
		return fmt.Errorf("content is required")
	}

	// validate filesystem
	if _, ok := state.Filesystem.(*filesystem.EditedFileSystem); !ok {
		return fmt.Errorf("put can only be executed in Web mode")
	}

	if input["filename"] != filesystem.FakeStdinFilename {
		//return Reply{map[string]interface{}{"reply": "Error", "message": fmt.Sprintf("put filename must be \"%s\"", filesystem.FakeStdinFilename)}},
		return fmt.Errorf("put filename must be \"%s\"", filesystem.FakeStdinFilename)
	}

	return nil
}

// -=-= Setdir =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

var setdirModeChk = "local|web"

func setdir(state *State, input map[string]interface{}) (Reply, error) {

	if err := setdirValidate(state, input); err != nil {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}
	// assuming everything is good?
	mode := input["mode"]
	state.Mode = mode.(string)

	// local mode? get directory and local filesystem
	if mode == "local" {
		state.Dir = input["directory"].(string)
		state.Filesystem = filesystem.NewLocalFileSystem()
	}

	// web mode? use edited filesystem
	if mode == "web" {
		state.Dir = "."
		state.Filesystem = filesystem.NewEditedFileSystem(
			filesystem.NewLocalFileSystem(),
			map[string]*text.EditSet{})
	}

	state.State = 2
	return Reply{map[string]interface{}{"reply": "OK"}}, nil
}

func setdirValidate(state *State, input map[string]interface{}) error {
	if state.State < 1 {
		return errors.New("State must be non-zero for \"setdir\" command")
	}

	// mode key?
	mode, found := input["mode"]
	if !found {
		err := errors.New("\"mode\" key is required")
		return err
	}

	// validate the mode value
	modeValidator := regexp.MustCompile(setdirModeChk)
	if valid := modeValidator.MatchString(mode.(string)); !valid {
		return errors.New("\"mode\" key must be \"web|local\"")
	}
	// check for directory key if mode == local
	if mode == "local" {
		if _, found := input["directory"]; !found {
			return errors.New("\"directory\" key required if \"mode\" is local")
		}
		// validate directory
		fs := filesystem.NewLocalFileSystem()
		_, err := fs.ReadDir(input["directory"].(string))
		if err != nil {
			return err
		}
	}
	return nil
}

// -=-= XRun =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

var xRunModeChk = "text|patch"

// TODO implement
func xRun(state *State, input map[string]interface{}) (Reply, error) {
	if err := xRunValidate(state, input); err != nil {
		return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
	}
	// setup text selection
	textselection := input["textselection"].(map[string]interface{})

	ts, _ := parseSelection(state, textselection)

	if ts.GetFilename() == filesystem.FakeStdinFilename {
		stdinPath, err := filesystem.FakeStdinPath()
		if err != nil {
			return Reply{map[string]interface{}{"reply": "Error",
				"message": err.Error()}}, err
		}
		switch ts := ts.(type) {
		case *text.OffsetLengthSelection:
			ts.Filename = stdinPath
		case *text.LineColSelection:
			ts.Filename = stdinPath
		}
	}

	// get refactoring
	refac := engine.GetRefactoring(input["transformation"].(string))

	config := &refactoring.Config{
		FileSystem: state.Filesystem,
		Scope:      nil,
		Selection:  ts,
		Args:       input["arguments"].([]interface{}),
	}

	// run
	result := refac.Run(config)

	// grab logs
	limit, found := input["limit"].(int)
	if !found || limit > len(result.Log.Entries) {
		limit = len(result.Log.Entries)
	}
	logs := make([]map[string]interface{}, 0)
	for _, entry := range result.Log.Entries[:limit] {
		var severity string
		switch entry.Severity {
		case refactoring.Info:
			// No prefix
		case refactoring.Warning:
			severity = "warning"
		case refactoring.Error:
			severity = "error"
		}
		log := map[string]interface{}{"severity": severity, "message": entry.Message}
		logs = append(logs, log)
	}

	changes := make([]map[string]string, 0)

	// if mode == patch or no mode was given
	if mode, found := input["mode"]; !found || mode.(string) == "patch" {
		for f, e := range result.Edits {
			var p *text.Patch
			var err error
			p, err = filesystem.CreatePatch(e, state.Filesystem, f)
			if err != nil {
				return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
			}
			diffFile, err := os.Create(strings.Join([]string{f, ".diff"}, ""))
			p.Write(f, f, time.Time{}, time.Time{}, diffFile)
			changes = append(changes, map[string]string{"filename": f, "patchFile": diffFile.Name()})
			diffFile.Close()
		}
	} else {
		for f, e := range result.Edits {
			content, err := filesystem.ApplyEdits(e, state.Filesystem, f)
			if err != nil {
				return Reply{map[string]interface{}{"reply": "Error", "message": err.Error()}}, err
			}
			changes = append(changes, map[string]string{"filename": f, "content": string(content)})
		}
	}

	// return without filesystem changes
	return Reply{map[string]interface{}{"reply": "OK", "description": refac.Description().Name, "log": logs, "files": changes}}, nil
}

// TODO validate TextSelection, FileSelection, arguments
func xRunValidate(state *State, input map[string]interface{}) error {
	if state.State < 2 {
		return errors.New("State of 2 (file system configured) is required")
	}

	// check transformation is valid
	if engine.GetRefactoring(input["transformation"].(string)) == nil {
		return errors.New("Transformation given is not a valid refactoring name")
	}

	// validate text/file selection
	// TODO validate fileselection
	textselection, tsfound := input["textselection"]
	_, fsfound := input["fileselection"]

	if tsfound && fsfound {
		return errors.New("Both textselection and fileselection cannot be used together")
	} else if tsfound {
		_, err := parseSelection(state, textselection.(map[string]interface{}))
		if err != nil {
			return err
		}
	} else if fsfound {

	}

	// check limit is > 0 if exists
	if limit, found := input["limit"]; found {
		if limit.(int) < 0 {
			return errors.New("\"limit\" key must be a positive integer")
		}
	}

	// check mode key if exists
	if mode, found := input["mode"]; found {
		qualityValidator := regexp.MustCompile(xRunModeChk)

		if valid := qualityValidator.MatchString(mode.(string)); !valid {
			return errors.New("\"mode\" key must be \"text|patch\"")
		}
	}

	// all good?
	return nil
}

// -=-= Helpers =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// takes a map for a text selection, either in line/col form or offset/length
// and returns the appropriate type (LineColSelection or OffsetLengthSelection)
// also can be used to simply validate the text selection given
func parseSelection(state *State, input map[string]interface{}) (text.Selection, error) {
	// validate filename
	filename, filefound := input["filename"]
	if !filefound {
		return nil, fmt.Errorf("File is not given")
	}
	if reflect.TypeOf(input["filename"]).Kind() != reflect.String {
		return nil, fmt.Errorf("Invalid type of value given for file: given %T", reflect.TypeOf(input["filename"]))
	}
	file := filepath.Join(state.Dir, filename.(string))

	// determine if offset/length or line/col
	offset, offsetFound := input["offset"]
	length, lengthFound := input["length"]
	if offsetFound && lengthFound {
		// validate
		if reflect.TypeOf(offset).Kind() != reflect.Float64 ||
			reflect.TypeOf(length).Kind() != reflect.Float64 {
			return nil, fmt.Errorf("Invalid type(s) given for offset/length combo (%v, %v)", reflect.TypeOf(offset), reflect.TypeOf(length))
		}

		pos := fmt.Sprintf("%d,%d", int(offset.(float64)), int(length.(float64)))
		ts, err := text.NewSelection(file, pos)
		if err != nil {
			return nil, err
		}
		return ts, nil
	}

	sl, slfound := input["startline"]
	sc, scfound := input["startcol"]
	el, elfound := input["endline"]
	ec, ecfound := input["endcol"]

	if slfound && scfound && elfound && ecfound {
		// validate
		if reflect.TypeOf(sl).Kind() != reflect.Float64 ||
			reflect.TypeOf(sc).Kind() != reflect.Float64 ||
			reflect.TypeOf(el).Kind() != reflect.Float64 ||
			reflect.TypeOf(ec).Kind() != reflect.Float64 {
			return nil, fmt.Errorf("invalid type(s) given for line/col combo")
		}
		pos := fmt.Sprintf("%d,%d:%d,%d", int(sl.(float64)), int(sc.(float64)), int(el.(float64)), int(ec.(float64)))
		ts, err := text.NewSelection(file, pos)
		if err != nil {
			return nil, err
		}
		return ts, nil
	}

	return nil, fmt.Errorf("invalid selection (offset/length or line/col")
}
