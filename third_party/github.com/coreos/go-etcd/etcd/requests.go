package etcd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"
)

// Valid options for GET, PUT, POST, DELETE
// Using CAPITALIZED_UNDERSCORE to emphasize that these
// values are meant to be used as constants.
var (
	VALID_GET_OPTIONS = validOptions{
		"recursive":  reflect.Bool,
		"consistent": reflect.Bool,
		"sorted":     reflect.Bool,
		"wait":       reflect.Bool,
		"waitIndex":  reflect.Uint64,
	}

	VALID_PUT_OPTIONS = validOptions{
		"prevValue": reflect.String,
		"prevIndex": reflect.Uint64,
		"prevExist": reflect.Bool,
	}

	VALID_POST_OPTIONS = validOptions{}

	VALID_DELETE_OPTIONS = validOptions{
		"recursive": reflect.Bool,
	}

	curlChan chan string
)

// SetCurlChan sets a channel to which cURL commands which can be used to
// re-produce requests are sent.  This is useful for debugging.
func SetCurlChan(c chan string) {
	curlChan = c
}

// get issues a GET request
func (c *Client) get(key string, options options) (*Response, error) {
	logger.Debugf("get %s [%s]", key, c.cluster.Leader)

	p := path.Join("keys", key)
	// If consistency level is set to STRONG, append
	// the `consistent` query string.
	if c.config.Consistency == STRONG_CONSISTENCY {
		options["consistent"] = true
	}
	if options != nil {
		str, err := optionsToString(options, VALID_GET_OPTIONS)
		if err != nil {
			return nil, err
		}
		p += str
	}

	resp, err := c.sendRequest("GET", p, url.Values{})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// put issues a PUT request
func (c *Client) put(key string, value string, ttl uint64, options options) (*Response, error) {
	logger.Debugf("put %s, %s, ttl: %d, [%s]", key, value, ttl, c.cluster.Leader)
	v := url.Values{}

	if value != "" {
		v.Set("value", value)
	}

	if ttl > 0 {
		v.Set("ttl", fmt.Sprintf("%v", ttl))
	}

	p := path.Join("keys", key)
	if options != nil {
		str, err := optionsToString(options, VALID_PUT_OPTIONS)
		if err != nil {
			return nil, err
		}
		p += str
	}

	resp, err := c.sendRequest("PUT", p, v)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// post issues a POST request
func (c *Client) post(key string, value string, ttl uint64) (*Response, error) {
	logger.Debugf("post %s, %s, ttl: %d, [%s]", key, value, ttl, c.cluster.Leader)
	v := url.Values{}

	if value != "" {
		v.Set("value", value)
	}

	if ttl > 0 {
		v.Set("ttl", fmt.Sprintf("%v", ttl))
	}

	resp, err := c.sendRequest("POST", path.Join("keys", key), v)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// delete issues a DELETE request
func (c *Client) delete(key string, options options) (*Response, error) {
	logger.Debugf("delete %s [%s]", key, c.cluster.Leader)
	v := url.Values{}

	p := path.Join("keys", key)
	if options != nil {
		str, err := optionsToString(options, VALID_DELETE_OPTIONS)
		if err != nil {
			return nil, err
		}
		p += str
	}

	resp, err := c.sendRequest("DELETE", p, v)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// sendRequest sends a HTTP request and returns a Response as defined by etcd
func (c *Client) sendRequest(method string, _path string, values url.Values) (*Response, error) {
	var body string = values.Encode()
	var resp *http.Response
	var req *http.Request

	retry := 0
	// if we connect to a follower, we will retry until we found a leader
	for {
		var httpPath string

		// If _path has schema already, then it's assumed to be
		// a complete URL and therefore needs no further processing.
		u, err := url.Parse(_path)
		if err != nil {
			return nil, err
		}

		if u.Scheme != "" {
			httpPath = _path
		} else {
			if method == "GET" && c.config.Consistency == WEAK_CONSISTENCY {
				// If it's a GET and consistency level is set to WEAK,
				// then use a random machine.
				httpPath = c.getHttpPath(true, _path)
			} else {
				// Else use the leader.
				httpPath = c.getHttpPath(false, _path)
			}
		}

		// Return a cURL command if curlChan is set
		if curlChan != nil {
			command := fmt.Sprintf("curl -X %s %s", method, httpPath)
			for key, value := range values {
				command += fmt.Sprintf(" -d %s=%s", key, value[0])
			}
			curlChan <- command
		}

		logger.Debug("send.request.to ", httpPath, " | method ", method)
		if body == "" {

			req, _ = http.NewRequest(method, httpPath, nil)

		} else {
			req, _ = http.NewRequest(method, httpPath, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
		}

		resp, err = c.httpClient.Do(req)

		logger.Debug("recv.response.from ", httpPath)
		// network error, change a machine!
		if err != nil {
			retry++
			if retry > 2*len(c.cluster.Machines) {
				return nil, errors.New("Cannot reach servers")
			}
			num := retry % len(c.cluster.Machines)
			logger.Debug("update.leader[", c.cluster.Leader, ",", c.cluster.Machines[num], "]")
			c.cluster.Leader = c.cluster.Machines[num]
			time.Sleep(time.Millisecond * 200)
			continue
		}

		if resp != nil {
			if resp.StatusCode == http.StatusTemporaryRedirect {
				httpPath := resp.Header.Get("Location")

				resp.Body.Close()

				if httpPath == "" {
					return nil, errors.New("Cannot get redirection location")
				}

				c.updateLeader(httpPath)
				logger.Debug("send.redirect")
				// try to connect the leader
				continue
			} else if resp.StatusCode == http.StatusInternalServerError {
				resp.Body.Close()

				retry++
				if retry > 2*len(c.cluster.Machines) {
					return nil, errors.New("Cannot reach servers")
				}
				continue
			} else {
				logger.Debug("send.return.response ", httpPath)
				break
			}

		}
		logger.Debug("error.from ", httpPath, " ", err.Error())
		return nil, err
	}

	// Convert HTTP response to etcd response
	b, err := ioutil.ReadAll(resp.Body)

	resp.Body.Close()

	if err != nil {
		return nil, err
	}

	if !(resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusCreated) {
		return nil, handleError(b)
	}

	var result Response

	err = json.Unmarshal(b, &result)

	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) getHttpPath(random bool, s ...string) string {
	var machine string
	if random {
		machine = c.cluster.Machines[rand.Intn(len(c.cluster.Machines))]
	} else {
		machine = c.cluster.Leader
	}

	fullPath := machine + "/" + version
	for _, seg := range s {
		fullPath = fullPath + "/" + seg
	}

	return fullPath
}
