/*
Copyright 2014 CoreOS Inc.

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

package etcd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/coreos/etcd/config"
	etcdErr "github.com/coreos/etcd/error"
)

// v2client sends various requests using HTTP API.
// It is different from raft communication, and doesn't record anything in the log.
// The argument url is required to contain scheme and host only, and
// there is no trailing slash in it.
// Public functions return "etcd/error".Error intentionally to figure out
// etcd error code easily.
type v2client struct {
	stopped bool
	mu      sync.RWMutex

	http.Client
	wg sync.WaitGroup
}

func newClient(tc *tls.Config) *v2client {
	tr := new(http.Transport)
	tr.TLSClientConfig = tc
	return &v2client{Client: http.Client{Transport: tr}}
}

func (c *v2client) CloseConnections() {
	c.stop()
	c.wg.Wait()
	tr := c.Transport.(*http.Transport)
	tr.CloseIdleConnections()
}

// CheckVersion returns true when the version check on the server returns 200.
func (c *v2client) CheckVersion(url string, version int) (bool, *etcdErr.Error) {
	if c.runOne() == false {
		return false, clientError(errors.New("v2_client is stopped"))
	}
	defer c.finishOne()

	resp, err := c.Get(url + fmt.Sprintf("/version/%d/check", version))
	if err != nil {
		return false, clientError(err)
	}
	c.readBody(resp.Body)

	return resp.StatusCode == 200, nil
}

// GetVersion fetches the peer version of a cluster.
func (c *v2client) GetVersion(url string) (int, *etcdErr.Error) {
	if c.runOne() == false {
		return 0, clientError(errors.New("v2_client is stopped"))
	}
	defer c.finishOne()

	resp, err := c.Get(url + "/version")
	if err != nil {
		return 0, clientError(err)
	}

	body, err := c.readBody(resp.Body)
	if err != nil {
		return 0, clientError(err)
	}

	// Parse version number.
	version, err := strconv.Atoi(string(body))
	if err != nil {
		return 0, clientError(err)
	}
	return version, nil
}

func (c *v2client) GetMachines(url string) ([]*machineMessage, *etcdErr.Error) {
	if c.runOne() == false {
		return nil, clientError(errors.New("v2_client is stopped"))
	}
	defer c.finishOne()

	resp, err := c.Get(url + "/v2/admin/machines/")
	if err != nil {
		return nil, clientError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.readErrorBody(resp.Body)
	}

	msgs := new([]*machineMessage)
	if uerr := c.readJSONBody(resp.Body, msgs); uerr != nil {
		return nil, uerr
	}
	return *msgs, nil
}

func (c *v2client) GetClusterConfig(url string) (*config.ClusterConfig, *etcdErr.Error) {
	if c.runOne() == false {
		return nil, clientError(errors.New("v2_client is stopped"))
	}
	defer c.finishOne()

	resp, err := c.Get(url + "/v2/admin/config")
	if err != nil {
		return nil, clientError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.readErrorBody(resp.Body)
	}

	config := new(config.ClusterConfig)
	if uerr := c.readJSONBody(resp.Body, config); uerr != nil {
		return nil, uerr
	}
	return config, nil
}

// AddMachine adds machine to the cluster.
// The first return value is the commit index of join command.
func (c *v2client) AddMachine(url string, name string, info *context) *etcdErr.Error {
	if c.runOne() == false {
		return clientError(errors.New("v2_client is stopped"))
	}
	defer c.finishOne()

	b, _ := json.Marshal(info)
	url = url + "/v2/admin/machines/" + name

	resp, err := c.put(url, b)
	if err != nil {
		return clientError(err)
	}
	if resp.StatusCode != http.StatusOK {
		return c.readErrorBody(resp.Body)
	}
	c.readBody(resp.Body)
	return nil
}

func (c *v2client) readErrorBody(body io.ReadCloser) *etcdErr.Error {
	b, err := c.readBody(body)
	if err != nil {
		return clientError(err)
	}
	uerr := &etcdErr.Error{}
	if err := json.Unmarshal(b, uerr); err != nil {
		str := strings.TrimSpace(string(b))
		return etcdErr.NewError(etcdErr.EcodeClientInternal, str, 0)
	}
	return nil
}

func (c *v2client) readJSONBody(body io.ReadCloser, val interface{}) *etcdErr.Error {
	if err := json.NewDecoder(body).Decode(val); err != nil {
		return clientError(err)
	}
	c.readBody(body)
	return nil
}

func (c *v2client) readBody(body io.ReadCloser) ([]byte, error) {
	b, err := ioutil.ReadAll(body)
	body.Close()
	return b, err
}

// put sends server side PUT request.
// It always follows redirects instead of stopping according to RFC 2616.
func (c *v2client) put(urlStr string, body []byte) (*http.Response, error) {
	return c.doAlwaysFollowingRedirects("PUT", urlStr, body)
}

func (c *v2client) doAlwaysFollowingRedirects(method string, urlStr string, body []byte) (resp *http.Response, err error) {
	var req *http.Request

	for redirect := 0; redirect < 10; redirect++ {
		req, err = http.NewRequest(method, urlStr, bytes.NewBuffer(body))
		if err != nil {
			return
		}

		if resp, err = c.Do(req); err != nil {
			if resp != nil {
				resp.Body.Close()
			}
			return
		}

		if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusTemporaryRedirect {
			resp.Body.Close()
			if urlStr = resp.Header.Get("Location"); urlStr == "" {
				err = errors.New(fmt.Sprintf("%d response missing Location header", resp.StatusCode))
				return
			}
			continue
		}
		return
	}

	err = errors.New("stopped after 10 redirects")
	return
}

func (c *v2client) runOne() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.stopped {
		return false
	}
	c.wg.Add(1)
	return true
}

func (c *v2client) finishOne() {
	c.wg.Done()
}

func (c *v2client) stop() {
	c.mu.Lock()
	c.stopped = true
	c.mu.Unlock()
}

func clientError(err error) *etcdErr.Error {
	return etcdErr.NewError(etcdErr.EcodeClientInternal, err.Error(), 0)
}
