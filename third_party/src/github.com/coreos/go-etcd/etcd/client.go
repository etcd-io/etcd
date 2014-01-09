package etcd

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const (
	HTTP = iota
	HTTPS
)

// See SetConsistency for how to use these constants.
const (
	// Using strings rather than iota because the consistency level
	// could be persisted to disk, so it'd be better to use
	// human-readable values.
	STRONG_CONSISTENCY = "STRONG"
	WEAK_CONSISTENCY   = "WEAK"
)

const (
	defaultBufferSize = 10
)

type Cluster struct {
	Leader   string   `json:"leader"`
	Machines []string `json:"machines"`
}

type Config struct {
	CertFile    string        `json:"certFile"`
	KeyFile     string        `json:"keyFile"`
	CaCertFile  string        `json:"caCertFile"`
	Scheme      string        `json:"scheme"`
	Timeout     time.Duration `json:"timeout"`
	Consistency string        `json: "consistency"`
}

type Client struct {
	cluster     Cluster `json:"cluster"`
	config      Config  `json:"config"`
	httpClient  *http.Client
	persistence io.Writer
	cURLch      chan string
}

// NewClient create a basic client that is configured to be used
// with the given machine list.
func NewClient(machines []string) *Client {
	// if an empty slice was sent in then just assume localhost
	if len(machines) == 0 {
		machines = []string{"http://127.0.0.1:4001"}
	}

	// default leader and machines
	cluster := Cluster{
		Leader:   machines[0],
		Machines: machines,
	}

	config := Config{
		// default use http
		Scheme: "http",
		// default timeout is one second
		Timeout: time.Second,
		// default consistency level is STRONG
		Consistency: STRONG_CONSISTENCY,
	}

	client := &Client{
		cluster: cluster,
		config:  config,
	}

	err := setupHttpClient(client)
	if err != nil {
		panic(err)
	}

	return client
}

// NewClientFile creates a client from a given file path.
// The given file is expected to use the JSON format.
func NewClientFile(fpath string) (*Client, error) {
	fi, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := fi.Close(); err != nil {
			panic(err)
		}
	}()

	return NewClientReader(fi)
}

// NewClientReader creates a Client configured from a given reader.
// The config is expected to use the JSON format.
func NewClientReader(reader io.Reader) (*Client, error) {
	var client Client

	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, &client)
	if err != nil {
		return nil, err
	}

	err = setupHttpClient(&client)
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func setupHttpClient(client *Client) error {
	if client.config.CertFile != "" && client.config.KeyFile != "" {
		err := client.SetCertAndKey(client.config.CertFile, client.config.KeyFile, client.config.CaCertFile)
		if err != nil {
			return err
		}
	} else {
		client.config.CertFile = ""
		client.config.KeyFile = ""
		tr := &http.Transport{
			Dial: dialTimeout,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		client.httpClient = &http.Client{Transport: tr}
	}

	return nil
}

// SetPersistence sets a writer to which the config will be
// written every time it's changed.
func (c *Client) SetPersistence(writer io.Writer) {
	c.persistence = writer
}

// SetConsistency changes the consistency level of the client.
//
// When consistency is set to STRONG_CONSISTENCY, all requests,
// including GET, are sent to the leader.  This means that, assuming
// the absence of leader failures, GET requests are guaranteed to see
// the changes made by previous requests.
//
// When consistency is set to WEAK_CONSISTENCY, other requests
// are still sent to the leader, but GET requests are sent to a
// random server from the server pool.  This reduces the read
// load on the leader, but it's not guaranteed that the GET requests
// will see changes made by previous requests (they might have not
// yet been committed on non-leader servers).
func (c *Client) SetConsistency(consistency string) error {
	if !(consistency == STRONG_CONSISTENCY || consistency == WEAK_CONSISTENCY) {
		return errors.New("The argument must be either STRONG_CONSISTENCY or WEAK_CONSISTENCY.")
	}
	c.config.Consistency = consistency
	return nil
}

// MarshalJSON implements the Marshaller interface
// as defined by the standard JSON package.
func (c *Client) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(struct {
		Config  Config  `json:"config"`
		Cluster Cluster `json:"cluster"`
	}{
		Config:  c.config,
		Cluster: c.cluster,
	})

	if err != nil {
		return nil, err
	}

	return b, nil
}

// UnmarshalJSON implements the Unmarshaller interface
// as defined by the standard JSON package.
func (c *Client) UnmarshalJSON(b []byte) error {
	temp := struct {
		Config  Config  `json: "config"`
		Cluster Cluster `json: "cluster"`
	}{}
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}

	c.cluster = temp.Cluster
	c.config = temp.Config
	return nil
}

// saveConfig saves the current config using c.persistence.
func (c *Client) saveConfig() error {
	if c.persistence != nil {
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}

		_, err = c.persistence.Write(b)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) SetCertAndKey(cert string, key string, caCert string) error {
	if cert != "" && key != "" {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)

		if err != nil {
			return err
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		}

		if caCert != "" {
			caCertPool := x509.NewCertPool()

			certBytes, err := ioutil.ReadFile(caCert)
			if err != nil {
				return err
			}

			if !caCertPool.AppendCertsFromPEM(certBytes) {
				return errors.New("Unable to load caCert")
			}

			tlsConfig.RootCAs = caCertPool
		} else {
			tlsConfig.InsecureSkipVerify = true
		}

		tr := &http.Transport{
			TLSClientConfig: tlsConfig,
			Dial:            dialTimeout,
		}

		c.httpClient = &http.Client{Transport: tr}
		c.saveConfig()
		return nil
	}
	return errors.New("Require both cert and key path")
}

func (c *Client) SetScheme(scheme int) error {
	if scheme == HTTP {
		c.config.Scheme = "http"
		c.saveConfig()
		return nil
	}
	if scheme == HTTPS {
		c.config.Scheme = "https"
		c.saveConfig()
		return nil
	}
	return errors.New("Unknown Scheme")
}

// SetCluster updates config using the given machine list.
func (c *Client) SetCluster(machines []string) bool {
	success := c.internalSyncCluster(machines)
	return success
}

func (c *Client) GetCluster() []string {
	return c.cluster.Machines
}

// SyncCluster updates config using the internal machine list.
func (c *Client) SyncCluster() bool {
	success := c.internalSyncCluster(c.cluster.Machines)
	return success
}

// internalSyncCluster syncs cluster information using the given machine list.
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
			c.cluster.Machines = strings.Split(string(b), ", ")

			// update leader
			// the first one in the machine list is the leader
			logger.Debugf("update.leader[%s,%s]", c.cluster.Leader, c.cluster.Machines[0])
			c.cluster.Leader = c.cluster.Machines[0]

			logger.Debug("sync.machines ", c.cluster.Machines)
			c.saveConfig()
			return true
		}
	}
	return false
}

// createHttpPath creates a complete HTTP URL.
// serverName should contain both the host name and a port number, if any.
func (c *Client) createHttpPath(serverName string, _path string) string {
	u, _ := url.Parse(serverName)
	u.Path = path.Join(u.Path, "/", _path)

	if u.Scheme == "" {
		u.Scheme = "http"
	}
	return u.String()
}

// Dial with timeout.
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, time.Second)
}

func (c *Client) updateLeader(u *url.URL) {
	var leader string
	if u.Scheme == "" {
		leader = "http://" + u.Host
	} else {
		leader = u.Scheme + "://" + u.Host
	}

	logger.Debugf("update.leader[%s,%s]", c.cluster.Leader, leader)
	c.cluster.Leader = leader
	c.saveConfig()
}

// switchLeader switch the current leader to machines[num]
func (c *Client) switchLeader(num int) {
	logger.Debugf("switch.leader[from %v to %v]",
		c.cluster.Leader, c.cluster.Machines[num])

	c.cluster.Leader = c.cluster.Machines[num]
}

func (c *Client) OpenCURL() {
	c.cURLch = make(chan string, defaultBufferSize)
}

func (c *Client) CloseCURL() {
	c.cURLch = nil
}

func (c *Client) sendCURL(command string) {
	go func() {
		select {
		case c.cURLch <- command:
		default:
		}
	}()
}

func (c *Client) RecvCURL() string {
	return <-c.cURLch
}
