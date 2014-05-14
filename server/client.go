package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
)

// Client sends various requests using HTTP API.
// It is different from raft communication, and doesn't record anything in the log.
// The argument url is required to contain scheme and host only, and
// there is no trailing slash in it.
// Public functions return "etcd/error".Error intentionally to figure out
// etcd error code easily.
// TODO(yichengq): It is similar to go-etcd. But it could have many efforts
// to integrate the two. Leave it for further discussion.
type Client struct {
	http.Client
}

func NewClient(transport http.RoundTripper) *Client {
	return &Client{http.Client{Transport: transport}}
}

// CheckVersion returns true when the version check on the server returns 200.
func (c *Client) CheckVersion(url string, version int) (bool, *etcdErr.Error) {
	resp, err := c.Get(url + fmt.Sprintf("/version/%d/check", version))
	if err != nil {
		return false, clientError(err)
	}

	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

// GetVersion fetches the peer version of a cluster.
func (c *Client) GetVersion(url string) (int, *etcdErr.Error) {
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

func (c *Client) GetMachines(url string) ([]*machineMessage, *etcdErr.Error) {
	resp, err := c.Get(url + "/v2/admin/machines")
	if err != nil {
		return nil, clientError(err)
	}

	msgs := new([]*machineMessage)
	if uerr := c.parseJSONResponse(resp, msgs); uerr != nil {
		return nil, uerr
	}
	return *msgs, nil
}

func (c *Client) GetClusterConfig(url string) (*ClusterConfig, *etcdErr.Error) {
	resp, err := c.Get(url + "/v2/admin/config")
	if err != nil {
		return nil, clientError(err)
	}

	config := new(ClusterConfig)
	if uerr := c.parseJSONResponse(resp, config); uerr != nil {
		return nil, uerr
	}
	return config, nil
}

// AddMachine adds machine to the cluster.
// The first return value is the commit index of join command.
func (c *Client) AddMachine(url string, cmd *JoinCommand) (uint64, *etcdErr.Error) {
	b, _ := json.Marshal(cmd)
	url = url + "/join"

	log.Infof("Send Join Request to %s", url)
	resp, err := c.put(url, b)
	if err != nil {
		return 0, clientError(err)
	}
	defer resp.Body.Close()

	if err := c.checkErrorResponse(resp); err != nil {
		return 0, err
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, clientError(err)
	}
	index, numRead := binary.Uvarint(b)
	if numRead < 0 {
		return 0, clientError(fmt.Errorf("buf too small, or value too large"))
	}
	return index, nil
}

func (c *Client) parseJSONResponse(resp *http.Response, val interface{}) *etcdErr.Error {
	defer resp.Body.Close()

	if err := c.checkErrorResponse(resp); err != nil {
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(val); err != nil {
		log.Debugf("Error parsing join response: %v", err)
		return clientError(err)
	}
	return nil
}

func (c *Client) checkErrorResponse(resp *http.Response) *etcdErr.Error {
	if resp.StatusCode != http.StatusOK {
		uerr := &etcdErr.Error{}
		if err := json.NewDecoder(resp.Body).Decode(uerr); err != nil {
			log.Debugf("Error parsing response to etcd error: %v", err)
			return clientError(err)
		}
		return uerr
	}
	return nil
}

// put sends server side PUT request.
// It always follows redirects instead of stopping according to RFC 2616.
func (c *Client) put(urlStr string, body []byte) (*http.Response, error) {
	return c.doAlwaysFollowingRedirects("PUT", urlStr, body)
}

func (c *Client) doAlwaysFollowingRedirects(method string, urlStr string, body []byte) (resp *http.Response, err error) {
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
