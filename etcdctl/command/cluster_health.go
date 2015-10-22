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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

func NewClusterHealthCommand() cli.Command {
	return cli.Command{
		Name:  "cluster-health",
		Usage: "check the health of the etcd cluster",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: "forever", Usage: "forever check the health every 10 second until CTRL+C"},
		},
		Action: handleClusterHealth,
	}
}

func handleClusterHealth(c *cli.Context) {
	forever := c.Bool("forever")
	if forever {
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, os.Interrupt)

		go func() {
			<-sigch
			os.Exit(0)
		}()
	}

	tr, err := getTransport(c)
	if err != nil {
		handleError(ExitServerError, err)
	}

	hc := http.Client{
		Transport: tr,
	}

	cln := mustNewClientNoSync(c)
	mi := client.NewMembersAPI(cln)
	ms, err := mi.List(context.TODO())
	if err != nil {
		fmt.Println("cluster may be unhealthy: failed to list members")
		handleError(ExitServerError, err)
	}

	for {
		var health bool
		var wg sync.WaitGroup
		for _, m := range ms {
			wg.Add(1)
			go func(m client.Member) {
				if mhealth := checkmemberHealth(hc, m); mhealth {
					health = true
				}
				wg.Done()
			}(m)
		}
		wg.Wait()

		if health {
			fmt.Println("cluster is healthy")
		} else {
			fmt.Println("cluster is unhealthy")
		}

		if !forever {
			break
		}

		fmt.Printf("\nnext check after 10 second...\n\n")
		time.Sleep(10 * time.Second)
	}
}

func checkmemberHealth(hc http.Client, m client.Member) bool {
	if len(m.ClientURLs) == 0 {
		fmt.Printf("member %s is unreachable: no available published client urls\n", m.ID)
		return false
	}

	for _, url := range m.ClientURLs {
		resp, err := hc.Get(url + "/health")
		if err != nil {
			fmt.Printf("failed to check the health of member %s on %s: %v\n", m.ID, url, err)
			continue
		}

		result := struct{ Health string }{}
		nresult := struct{ Health bool }{}
		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("failed to check the health of member %s on %s: %v\n", m.ID, url, err)
			continue
		}
		resp.Body.Close()

		err = json.Unmarshal(bytes, &result)
		if err != nil {
			err = json.Unmarshal(bytes, &nresult)
		}
		if err != nil {
			fmt.Printf("failed to check the health of member %s on %s: %v\n", m.ID, url, err)
			continue
		}

		if result.Health == "true" || nresult.Health == true {
			fmt.Printf("member %s is healthy: got healthy result from %s\n", m.ID, url)
			return true
		} else {
			fmt.Printf("member %s is unhealthy: got unhealthy result from %s\n", m.ID, url)
			return false
		}
	}

	fmt.Printf("member %s is unreachable: %v are all unreachable\n", m.ID, m.ClientURLs)
	return false
}
