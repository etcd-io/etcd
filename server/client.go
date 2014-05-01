package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/log"
)

// Client sends various requests using HTTP API.
// It is different from raft communication, and doesn't record anything in the log.
// TODO(yichengq): It is similar to go-etcd. But it could have many efforts
// to integrate the two. Leave it for further discussion.
type Client struct {
	http.Client
}

func NewClient(transport http.RoundTripper) *Client {
	return &Client{http.Client{Transport: transport}}
}

// CheckVersion checks whether the version is available.
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
	version, _ := strconv.Atoi(string(body))
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
func (c *Client) AddMachine(url string, cmd *JoinCommandV2) (*joinResponseV2, *etcdErr.Error) {
	b, _ := json.Marshal(cmd)
	url = url + "/v2/admin/machines/" + cmd.Name

	log.Infof("Send Join Request to %s", url)
	resp, err := c.Put(url, b)
	if err != nil {
		return nil, clientError(err)
	}

	msg := new(joinResponseV2)
	if uerr := c.parseJSONResponse(resp, msg); uerr != nil {
		return nil, uerr
	}
	return msg, nil
}

func (c *Client) parseJSONResponse(resp *http.Response, val interface{}) *etcdErr.Error {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		uerr := &etcdErr.Error{}
		if err := json.NewDecoder(resp.Body).Decode(uerr); err != nil {
			log.Debugf("Error parsing etcd error: %v", err)
			return clientError(err)
		}
		return uerr
	}

	if err := json.NewDecoder(resp.Body).Decode(val); err != nil {
		log.Debugf("Error parsing join response: %v", err)
		return clientError(err)
	}
	return nil
}

// Put sends server side PUT request
func (c *Client) Put(urlStr string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("PUT", urlStr, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	return c.doAlwaysFollowingRedirects(req, body)
}

// doAlwaysFollowingRedirects provides similar functionality as standard one,
// but it does redirect with the same method for PUT or POST requests.
// Part of the code is borrowed from pkg/net/http/client.go.
func (c *Client) doAlwaysFollowingRedirects(ireq *http.Request, body []byte) (resp *http.Response, err error) {
	var base *url.URL
	redirectChecker := c.CheckRedirect
	if redirectChecker == nil {
		redirectChecker = defaultCheckRedirect
	}
	var via []*http.Request

	if ireq.URL == nil {
		return nil, errors.New("http: nil Request.URL")
	}

	req := ireq
	urlStr := "" // next relative or absolute URL to fetch (after first request)
	for redirect := 0; ; redirect++ {
		if redirect != 0 {
			req, err = http.NewRequest(ireq.Method, urlStr, bytes.NewBuffer(body))
			if err != nil {
				break
			}
			req.URL = base.ResolveReference(req.URL)
			if len(via) > 0 {
				// Add the Referer header.
				lastReq := via[len(via)-1]
				if lastReq.URL.Scheme != "https" {
					req.Header.Set("Referer", lastReq.URL.String())
				}

				err = redirectChecker(req, via)
				if err != nil {
					break
				}
			}
		}

		urlStr = req.URL.String()
		// It uses exported Do method here.
		// It is more elegant to use unexported send method, but that will
		// introduce many redundant code.
		if resp, err = c.Do(req); err != nil {
			break
		}

		if shouldExtraRedirectPost(resp.StatusCode) {
			resp.Body.Close()
			if urlStr = resp.Header.Get("Location"); urlStr == "" {
				err = errors.New(fmt.Sprintf("%d response missing Location header", resp.StatusCode))
				break
			}
			base = req.URL
			via = append(via, req)
			continue
		}
		return
	}

	method := ireq.Method
	urlErr := &url.Error{
		Op:  method[0:1] + strings.ToLower(method[1:]),
		URL: urlStr,
		Err: err,
	}

	if resp != nil {
		resp.Body.Close()
	}
	return nil, urlErr
}

func shouldExtraRedirectPost(statusCode int) bool {
	switch statusCode {
	case http.StatusMovedPermanently, http.StatusTemporaryRedirect:
		return true
	}
	return false
}

func defaultCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	return nil
}

func clientError(err error) *etcdErr.Error {
	return etcdErr.NewError(etcdErr.EcodeClientInternal, err.Error(), 0)
}
