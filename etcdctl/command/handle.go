// Copyright 2015 CoreOS, Inc.
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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/bgentry/speakeasy"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
)

type handlerFunc func(*cobra.Command, []string, *etcd.Client) (*etcd.Response, error)
type printFunc func(*etcd.Response, string)
type contextualPrintFunc func(*cobra.Command, *etcd.Response, string)

// dumpCURL blindly dumps all curl output to os.Stderr
func dumpCURL(client *etcd.Client) {
	client.OpenCURL()
	for {
		fmt.Fprintf(os.Stderr, "Curl-Example: %s\n", client.RecvCURL())
	}
}

func getUsernamePasswordFromFlag(usernameFlag string) (username string, password string, err error) {
	colon := strings.Index(usernameFlag, ":")
	if colon == -1 {
		username = usernameFlag
		// Prompt for the password.
		password, err = speakeasy.Ask("Password: ")
		if err != nil {
			return "", "", err
		}
	} else {
		username = usernameFlag[:colon]
		password = usernameFlag[colon+1:]
	}
	return username, password, nil
}

func prepAuth(client *etcd.Client, usernameFlag string) error {
	username, password, err := getUsernamePasswordFromFlag(usernameFlag)
	if err != nil {
		return err
	}
	client.SetCredentials(username, password)
	return nil
}

// rawhandle wraps the command function handlers and sets up the
// environment but performs no output formatting.
func rawhandle(cmd *cobra.Command, args []string, fn handlerFunc) (*etcd.Response, error) {
	endpoints, err := getEndpoints(cmd)
	if err != nil {
		return nil, err
	}

	tr, err := getTransport(cmd)
	if err != nil {
		return nil, err
	}

	client := etcd.NewClient(endpoints)
	client.SetTransport(tr)

	username, _ := cmd.Flags().GetString("username")
	if username != "" {
		err := prepAuth(client, username)
		if err != nil {
			return nil, err
		}
	}

	d, _ := cmd.Flags().GetBool("debug")
	if d {
		go dumpCURL(client)
	}

	// Sync cluster.
	if nosync, _ := cmd.Flags().GetBool("no-sync"); !nosync {
		if ok := client.SyncCluster(); !ok {
			handleError(ExitBadConnection, errors.New("cannot sync with the cluster using endpoints "+strings.Join(endpoints, ", ")))
		}
	}

	if d {
		fmt.Fprintf(os.Stderr, "Cluster-Endpoints: %s\n", strings.Join(client.GetCluster(), ", "))
	}

	// Execute handler function.
	return fn(cmd, args, client)
}

// handlePrint wraps the command function handlers to parse global flags
// into a client and to properly format the response objects.
func handlePrint(cmd *cobra.Command, args []string, fn handlerFunc, pFn printFunc) {
	resp, err := rawhandle(cmd, args, fn)

	// Print error and exit, if necessary.
	if err != nil {
		handleError(ExitServerError, err)
	}

	if resp != nil && pFn != nil {
		output, _ := cmd.Flags().GetString("output")
		pFn(resp, output)
	}
}

// Just like handlePrint but also passed the context of the command
func handleContextualPrint(cmd *cobra.Command, args []string, fn handlerFunc, pFn contextualPrintFunc) {
	resp, err := rawhandle(cmd, args, fn)

	if err != nil {
		handleError(ExitServerError, err)
	}

	if resp != nil && pFn != nil {
		output, _ := cmd.Flags().GetString("output")
		pFn(cmd, resp, output)
	}
}

// handleDir handles a request that wants to do operations on a single dir.
// Dir cannot be printed out, so we set NIL print function here.
func handleDir(cmd *cobra.Command, args []string, fn handlerFunc) {
	handlePrint(cmd, args, fn, nil)
}

// handleKey handles a request that wants to do operations on a single key.
func handleKey(cmd *cobra.Command, args []string, fn handlerFunc) {
	handlePrint(cmd, args, fn, printKey)
}

func handleAll(cmd *cobra.Command, args []string, fn handlerFunc) {
	handlePrint(cmd, args, fn, printAll)
}
