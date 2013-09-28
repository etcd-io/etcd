package etcd

import (
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	HTTP = iota
	HTTPS
)

type Cluster struct {
	Leader   string
	Machines []string
}

type Config struct {
	CertFile string
	KeyFile  string
	Scheme   string
	Timeout  time.Duration
}

type Client struct {
	cluster    Cluster
	config     Config
	httpClient *http.Client
}

// Setup a basic conf and cluster
func NewClient() *Client {

	// default leader and machines
	cluster := Cluster{
		Leader:   "http://127.0.0.1:4001",
		Machines: []string{"http://127.0.0.1:4001"},
	}

	config := Config{
		// default use http
		Scheme: "http",
		// default timeout is one second
		Timeout: time.Second,
	}

	tr := &http.Transport{
		Dial: dialTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return &Client{
		cluster:    cluster,
		config:     config,
		httpClient: &http.Client{Transport: tr},
	}

}

func (c *Client) SetCertAndKey(cert string, key string) (bool, error) {

	if cert != "" && key != "" {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)

		if err != nil {
			return false, err
		}

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{tlsCert},
				InsecureSkipVerify: true,
			},
			Dial: dialTimeout,
		}

		c.httpClient = &http.Client{Transport: tr}
		return true, nil
	}
	return false, errors.New("Require both cert and key path")
}

func (c *Client) SetScheme(scheme int) (bool, error) {
	if scheme == HTTP {
		c.config.Scheme = "http"
		return true, nil
	}
	if scheme == HTTPS {
		c.config.Scheme = "https"
		return true, nil
	}
	return false, errors.New("Unknown Scheme")
}

// Try to sync from the given machine
func (c *Client) SetCluster(machines []string) bool {
	success := c.internalSyncCluster(machines)
	return success
}

// sycn cluster information using the existing machine list
func (c *Client) SyncCluster() bool {
	success := c.internalSyncCluster(c.cluster.Machines)
	return success
}

// sync cluster information by providing machine list
func (c *Client) internalSyncCluster(machines []string) bool {
	for _, machine := range machines {
		httpPath := c.createHttpPath(machine, version+"/machines")
		resp, err := c.httpClient.Get(httpPath)
		if err != nil {
			// try another machine in the cluster
			continue
		} else {
			b, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				// try another machine in the cluster
				continue
			}
			// update Machines List
			c.cluster.Machines = strings.Split(string(b), ",")

			// update leader
			// the first one in the machine list is the leader
			logger.Debugf("update.leader[%s,%s]", c.cluster.Leader, c.cluster.Machines[0])
			c.cluster.Leader = c.cluster.Machines[0]

			logger.Debug("sync.machines ", c.cluster.Machines)
			return true
		}
	}
	return false
}

// serverName should contain both hostName and port
func (c *Client) createHttpPath(serverName string, _path string) string {
	u, _ := url.Parse(serverName)
	u.Path = path.Join(u.Path, "/", _path)
	return u.String()
}

// Dial with timeout.
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, time.Second)
}

func (c *Client) getHttpPath(s ...string) string {
	u, _ := url.Parse(c.cluster.Leader)

	u.Path = path.Join(u.Path, "/", version)

	for _, seg := range s {
		u.Path = path.Join(u.Path, seg)
	}

	return u.String()
}

func (c *Client) updateLeader(httpPath string) {
	u, _ := url.Parse(httpPath)

	var leader string
	if u.Scheme == "" {
		leader = "http://" + u.Host
	} else {
		leader = u.Scheme + "://" + u.Host
	}

	logger.Debugf("update.leader[%s,%s]", c.cluster.Leader, leader)
	c.cluster.Leader = leader
}

// Wrap GET, POST and internal error handling
func (c *Client) sendRequest(method string, _path string, body string) (*http.Response, error) {

	var resp *http.Response
	var err error
	var req *http.Request

	retry := 0
	// if we connect to a follower, we will retry until we found a leader
	for {

		httpPath := c.getHttpPath(_path)

		logger.Debug("send.request.to ", httpPath)
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
	return resp, nil
}
