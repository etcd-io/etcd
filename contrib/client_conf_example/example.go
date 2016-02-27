// Copyright 2016 Nippon Telegraph and Telephone Corporation.
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

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

var cmd = &cobra.Command{
	Use:   "example",
	Short: "An example of file and environment variable based client configuration",
	Run:   runExample,
}

var (
	confFilePath string
	verbose      bool
	modeGet      bool
)

func init() {
	cmd.Flags().StringVar(&confFilePath, "conf-file", "", "a path of configuration (if it is empty, env vars are used)")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "be verbose")
	cmd.Flags().BoolVar(&modeGet, "get", false, "execute get requests instead of put requests")
}

func runExample(cmd *cobra.Command, args []string) {
	cfg := client.Config{
		Transport: client.DefaultTransport,
	}

	var cli client.Client
	var err error

	if confFilePath == "" {
		if verbose {
			fmt.Printf("--conf-file option isn't passed. Using envronment variables\n")
		}
		cli, err = client.NewWithEnv(cfg)
	} else {
		if verbose {
			fmt.Printf("using a configuration file: %s\n", confFilePath)
		}
		cli, err = client.NewWithFile(cfg, confFilePath)
	}

	if err != nil {
		fmt.Printf("failed to create client: %s\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("created client: %v\n", cli)
	}

	kapi := client.NewKeysAPI(cli)

	if !modeGet {
		for i := 0; i < 10; i++ {
			k := fmt.Sprintf("/k%d", i)
			v := fmt.Sprintf("v%d", i)
			_, err := kapi.Set(context.Background(), k, v, nil)
			if err != nil {
				fmt.Printf("failed to set %s to key %s, exiting\n", v, k)
				os.Exit(1)
			}
		}

		if verbose {
			fmt.Printf("keys are set correctly\n")
		}
	} else {
		for i := 0; i < 10; i++ {
			k := fmt.Sprintf("/k%d", i)
			resp, err := kapi.Get(context.Background(), k, nil)
			if err != nil {
				fmt.Printf("failed to get key %s, exiting\n", k)
				os.Exit(1)
			}

			v := fmt.Sprintf("v%d", i)
			if strings.Compare(resp.Node.Value, v) != 0 {
				fmt.Printf("key %s doesn't have an assumed value: %v (actual value: %s)\n", k, v, resp.Node.Value)
				os.Exit(1)
			}
		}

		if verbose {
			fmt.Printf("keys are got correctly\n")
		}
	}
}

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}
