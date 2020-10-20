// Copyright 2015 The etcd Authors
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

package command

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	pb "go.etcd.io/etcd/api/v3/mvccpb"
	v3 "go.etcd.io/etcd/client/v3"

	"github.com/spf13/cobra"
)

func printKV(isHex bool, valueOnly bool, kv *pb.KeyValue) {
	k, v := string(kv.Key), string(kv.Value)
	if isHex {
		k = addHexPrefix(hex.EncodeToString(kv.Key))
		v = addHexPrefix(hex.EncodeToString(kv.Value))
	}
	if !valueOnly {
		fmt.Println(k)
	}
	fmt.Println(v)
}

func addHexPrefix(s string) string {
	ns := make([]byte, len(s)*2)
	for i := 0; i < len(s); i += 2 {
		ns[i*2] = '\\'
		ns[i*2+1] = 'x'
		ns[i*2+2] = s[i]
		ns[i*2+3] = s[i+1]
	}
	return string(ns)
}

func argify(s string) []string {
	r := regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'[^']*'|[^'"\s]\S*[^'"\s]?`)
	args := r.FindAllString(s, -1)
	for i := range args {
		if len(args[i]) == 0 {
			continue
		}
		if args[i][0] == '\'' {
			// 'single-quoted string'
			args[i] = args[i][1 : len(args)-1]
		} else if args[i][0] == '"' {
			// "double quoted string"
			if _, err := fmt.Sscanf(args[i], "%q", &args[i]); err != nil {
				ExitWithError(ExitInvalidInput, err)
			}
		}
	}
	return args
}

func commandCtx(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	timeOut, err := cmd.Flags().GetDuration("command-timeout")
	if err != nil {
		ExitWithError(ExitError, err)
	}
	return context.WithTimeout(context.Background(), timeOut)
}

func isCommandTimeoutFlagSet(cmd *cobra.Command) bool {
	commandTimeoutFlag := cmd.Flags().Lookup("command-timeout")
	if commandTimeoutFlag == nil {
		panic("expect command-timeout flag to exist")
	}
	return commandTimeoutFlag.Changed
}

// get the process_resident_memory_bytes from <server:2379>/metrics
func endpointMemoryMetrics(host string) float64 {
	residentMemoryKey := "process_resident_memory_bytes"
	var residentMemoryValue string
	if !strings.HasPrefix(host, `http://`) {
		host = "http://" + host
	}
	url := host + "/metrics"
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(fmt.Sprintf("fetch error: %v", err))
		return 0.0
	}
	byts, readerr := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if readerr != nil {
		fmt.Println(fmt.Sprintf("fetch error: reading %s: %v", url, readerr))
		return 0.0
	}

	for _, line := range strings.Split(string(byts), "\n") {
		if strings.HasPrefix(line, residentMemoryKey) {
			residentMemoryValue = strings.TrimSpace(strings.TrimPrefix(line, residentMemoryKey))
			break
		}
	}
	if residentMemoryValue == "" {
		fmt.Println(fmt.Sprintf("could not find: %v", residentMemoryKey))
		return 0.0
	}
	residentMemoryBytes, parseErr := strconv.ParseFloat(residentMemoryValue, 64)
	if parseErr != nil {
		fmt.Println(fmt.Sprintf("parse error: %v", parseErr))
		return 0.0
	}

	return residentMemoryBytes
}

// compact keyspace history to a provided revision
func compact(c *v3.Client, rev int64) {
	fmt.Printf("Compacting with revision %d\n", rev)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, err := c.Compact(ctx, rev, v3.WithCompactPhysical())
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	fmt.Printf("Compacted with revision %d\n", rev)
}

// defrag a given endpoint
func defrag(c *v3.Client, ep string) {
	fmt.Printf("Defragmenting %q\n", ep)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, err := c.Defragment(ctx, ep)
	cancel()
	if err != nil {
		ExitWithError(ExitError, err)
	}
	fmt.Printf("Defragmented %q\n", ep)
}
