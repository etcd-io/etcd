package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

func parseInfo(path string) *Info {
	file, err := os.Open(path)

	if err != nil {
		return nil
	}
	defer file.Close()

	info := &Info{}

	content, err := ioutil.ReadAll(file)
	if err != nil {
		fatalf("Unable to read info: %v", err)
		return nil
	}

	if err = json.Unmarshal(content, &info); err != nil {
		fatalf("Unable to parse info: %v", err)
		return nil
	}

	return info
}

// Get the server info from previous conf file
// or from the user
func getInfo(path string) *Info {

	// Read in the server info if available.
	infoPath := filepath.Join(path, "info")

	// Delete the old configuration if exist
	if force {
		logPath := filepath.Join(path, "log")
		confPath := filepath.Join(path, "conf")
		snapshotPath := filepath.Join(path, "snapshot")
		os.Remove(infoPath)
		os.Remove(logPath)
		os.Remove(confPath)
		os.RemoveAll(snapshotPath)
	}

	info := parseInfo(infoPath)
	if info != nil {
		infof("Found node configuration in '%s'. Ignoring flags", infoPath)
		return info
	}

	info = &argInfo

	// Write to file.
	content, _ := json.MarshalIndent(info, "", " ")
	content = []byte(string(content) + "\n")
	if err := ioutil.WriteFile(infoPath, content, 0644); err != nil {
		fatalf("Unable to write info to file: %v", err)
	}

	infof("Wrote node configuration to '%s'", infoPath)

	return info
}
