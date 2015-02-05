/*
   Copyright 2015 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package command

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/pkg/transport"
)

func UpgradeCommand() cli.Command {
	return cli.Command{
		Name:  "upgrade",
		Usage: "upgrade an old version etcd cluster to a new version",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "old-version", Value: "1", Usage: "Old internal version"},
			cli.StringFlag{Name: "new-version", Value: "2", Usage: "New internal version"},
			cli.StringFlag{Name: "peer-url", Value: "", Usage: "An etcd peer url string"},
			cli.StringFlag{Name: "peer-cert-file", Value: "", Usage: "identify HTTPS peer using this SSL certificate file"},
			cli.StringFlag{Name: "peer-key-file", Value: "", Usage: "identify HTTPS peer using this SSL key file"},
			cli.StringFlag{Name: "peer-ca-file", Value: "", Usage: "verify certificates of HTTPS-enabled peers using this CA bundle"},
		},
		Action: handleUpgrade,
	}
}

func handleUpgrade(c *cli.Context) {
	if c.String("old-version") != "1" {
		fmt.Printf("Do not support upgrade from version %s\n", c.String("old-version"))
		os.Exit(1)
	}
	if c.String("new-version") != "2" {
		fmt.Printf("Do not support upgrade to version %s\n", c.String("new-version"))
		os.Exit(1)
	}
	tls := transport.TLSInfo{
		CAFile:   c.String("peer-ca-file"),
		CertFile: c.String("peer-cert-file"),
		KeyFile:  c.String("peer-key-file"),
	}
	t, err := transport.NewTransport(tls)
	if err != nil {
		log.Fatal(err)
	}
	client := http.Client{Transport: t}
	resp, err := client.Get(c.String("peer-url") + "/v2/admin/next-internal-version")
	if err != nil {
		fmt.Printf("Failed to send upgrade request to %s: %v\n", c.String("peer-url"), err)
		return
	}
	if resp.StatusCode == http.StatusOK {
		fmt.Println("Cluster will start upgrading from internal version 1 to 2 in 10 seconds.")
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("Cluster cannot upgrade to 2: version is not 0.4.7")
		return
	}
	fmt.Printf("Faild to send upgrade request to %s: bad status code %d\n", c.String("cluster-url"), resp.StatusCode)
}
