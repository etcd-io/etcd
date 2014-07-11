package etcd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

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
	http.Client
}

func newClient(tc *tls.Config) *v2client {
	tr := new(http.Transport)
	tr.TLSClientConfig = tc
	return &v2client{http.Client{Transport: tr}}
}

// CheckVersion returns true when the version check on the server returns 200.
func (c *v2client) CheckVersion(url string, version int) (bool, *etcdErr.Error) {
	resp, err := c.Get(url + fmt.Sprintf("/version/%d/check", version))
	if err != nil {
		return false, clientError(err)
	}

	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

// GetVersion fetches the peer version of a cluster.
func (c *v2client) GetVersion(url string) (int, *etcdErr.Error) {
	resp, err := c.Get(url + "/version")
	if err != nil {
		return 0, clientError(err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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
	resp, err := c.Get(url + "/v2/admin/machines/")
	if err != nil {
		return nil, clientError(err)
	}

	msgs := new([]*machineMessage)
	if uerr := c.parseJSONResponse(resp, msgs); uerr != nil {
		return nil, uerr
	}
	return *msgs, nil
}

func (c *v2client) GetClusterConfig(url string) (*config.ClusterConfig, *etcdErr.Error) {
	resp, err := c.Get(url + "/v2/admin/config")
	if err != nil {
		return nil, clientError(err)
	}

	config := new(config.ClusterConfig)
	if uerr := c.parseJSONResponse(resp, config); uerr != nil {
		return nil, uerr
	}
	return config, nil
}

// AddMachine adds machine to the cluster.
// The first return value is the commit index of join command.
func (c *v2client) AddMachine(url string, name string, info *context) *etcdErr.Error {
	b, _ := json.Marshal(info)
	url = url + "/v2/admin/machines/" + name

	log.Printf("Send Join Request to %s", url)
	resp, err := c.put(url, b)
	if err != nil {
		return clientError(err)
	}
	defer resp.Body.Close()

	if err := c.checkErrorResponse(resp); err != nil {
		return err
	}
	return nil
}

func (c *v2client) parseJSONResponse(resp *http.Response, val interface{}) *etcdErr.Error {
	defer resp.Body.Close()

	if err := c.checkErrorResponse(resp); err != nil {
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(val); err != nil {
		log.Printf("Error parsing join response: %v", err)
		return clientError(err)
	}
	return nil
}

func (c *v2client) checkErrorResponse(resp *http.Response) *etcdErr.Error {
	if resp.StatusCode != http.StatusOK {
		uerr := &etcdErr.Error{}
		if err := json.NewDecoder(resp.Body).Decode(uerr); err != nil {
			log.Printf("Error parsing response to etcd error: %v", err)
			return clientError(err)
		}
		return uerr
	}
	return nil
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

func clientError(err error) *etcdErr.Error {
	return etcdErr.NewError(etcdErr.EcodeClientInternal, err.Error(), 0)
}
