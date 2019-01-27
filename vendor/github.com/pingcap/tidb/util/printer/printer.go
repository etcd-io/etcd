// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package printer

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/config"
	"github.com/pingcap/tidb/util/israce"
	log "github.com/sirupsen/logrus"
)

// Version information.
var (
	TiDBBuildTS   = "None"
	TiDBGitHash   = "None"
	TiDBGitBranch = "None"
	GoVersion     = "None"
	// TiKVMinVersion is the minimum version of TiKV that can be compatible with the current TiDB.
	TiKVMinVersion = "2.1.0-alpha.1-ff3dd160846b7d1aed9079c389fc188f7f5ea13e"
)

// PrintTiDBInfo prints the TiDB version information.
func PrintTiDBInfo() {
	log.Infof("Welcome to TiDB.")
	log.Infof("Release Version: %s", mysql.TiDBReleaseVersion)
	log.Infof("Git Commit Hash: %s", TiDBGitHash)
	log.Infof("Git Branch: %s", TiDBGitBranch)
	log.Infof("UTC Build Time:  %s", TiDBBuildTS)
	log.Infof("GoVersion:  %s", GoVersion)
	log.Infof("Race Enabled: %v", israce.RaceEnabled)
	log.Infof("Check Table Before Drop: %v", config.CheckTableBeforeDrop)
	log.Infof("TiKV Min Version: %s", TiKVMinVersion)
	configJSON, err := json.Marshal(config.GetGlobalConfig())
	if err != nil {
		panic(err)
	}
	log.Infof("Config: %s", configJSON)
}

// GetTiDBInfo returns the git hash and build time of this tidb-server binary.
func GetTiDBInfo() string {
	return fmt.Sprintf("Release Version: %s\n"+
		"Git Commit Hash: %s\n"+
		"Git Branch: %s\n"+
		"UTC Build Time: %s\n"+
		"GoVersion: %s\n"+
		"Race Enabled: %v\n"+
		"TiKV Min Version: %s\n"+
		"Check Table Before Drop: %v",
		mysql.TiDBReleaseVersion,
		TiDBGitHash,
		TiDBGitBranch,
		TiDBBuildTS,
		GoVersion,
		israce.RaceEnabled,
		TiKVMinVersion,
		config.CheckTableBeforeDrop)
}

// checkValidity checks whether cols and every data have the same length.
func checkValidity(cols []string, datas [][]string) bool {
	colLen := len(cols)
	if len(datas) == 0 || colLen == 0 {
		return false
	}

	for _, data := range datas {
		if colLen != len(data) {
			return false
		}
	}

	return true
}

func getMaxColLen(cols []string, datas [][]string) []int {
	maxColLen := make([]int, len(cols))
	for i, col := range cols {
		maxColLen[i] = len(col)
	}

	for _, data := range datas {
		for i, v := range data {
			if len(v) > maxColLen[i] {
				maxColLen[i] = len(v)
			}
		}
	}

	return maxColLen
}

func getPrintDivLine(maxColLen []int) []byte {
	var value = make([]byte, 0)
	for _, v := range maxColLen {
		value = append(value, '+')
		value = append(value, bytes.Repeat([]byte{'-'}, v+2)...)
	}
	value = append(value, '+')
	value = append(value, '\n')
	return value
}

func getPrintCol(cols []string, maxColLen []int) []byte {
	var value = make([]byte, 0)
	for i, v := range cols {
		value = append(value, '|')
		value = append(value, ' ')
		value = append(value, []byte(v)...)
		value = append(value, bytes.Repeat([]byte{' '}, maxColLen[i]+1-len(v))...)
	}
	value = append(value, '|')
	value = append(value, '\n')
	return value
}

func getPrintRow(data []string, maxColLen []int) []byte {
	var value = make([]byte, 0)
	for i, v := range data {
		value = append(value, '|')
		value = append(value, ' ')
		value = append(value, []byte(v)...)
		value = append(value, bytes.Repeat([]byte{' '}, maxColLen[i]+1-len(v))...)
	}
	value = append(value, '|')
	value = append(value, '\n')
	return value
}

func getPrintRows(datas [][]string, maxColLen []int) []byte {
	var value = make([]byte, 0)
	for _, data := range datas {
		value = append(value, getPrintRow(data, maxColLen)...)
	}
	return value
}

// GetPrintResult gets a result with a formatted string.
func GetPrintResult(cols []string, datas [][]string) (string, bool) {
	if !checkValidity(cols, datas) {
		return "", false
	}

	var value = make([]byte, 0)
	maxColLen := getMaxColLen(cols, datas)

	value = append(value, getPrintDivLine(maxColLen)...)
	value = append(value, getPrintCol(cols, maxColLen)...)
	value = append(value, getPrintDivLine(maxColLen)...)
	value = append(value, getPrintRows(datas, maxColLen)...)
	value = append(value, getPrintDivLine(maxColLen)...)
	return string(value), true
}
