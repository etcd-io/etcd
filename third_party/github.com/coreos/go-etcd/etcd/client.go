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
	"time"
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

type Config struct {
	CertFile    string        `json:"certFile"`
	KeyFile     string        `json:"keyFile"`
	CaCertFile  []string      `json:"caCertFiles"`
	Timeout     time.Duration `json:"timeout"`
	Consistency string        `json: "consistency"`
}

type Client struct {
	config      Config   `json:"config"`
	cluster     *Cluster `json:"cluster"`
	httpClient  *http.Client
	persistence io.Writer
	cURLch      chan string
	keyPrefix   string
}

// NewClient create a basic client that is configured to be used
// with the given machine list.
func NewClient(machines []string) *Client {
	config := Config{
		// default timeout is one second
		Timeout: time.Second,
		// default consistency level is STRONG
		Consistency: STRONG_CONSISTENCY,
	}

	client := &Client{
		cluster:   NewCluster(machines),
		config:    config,
		keyPrefix: path.Join(version, "keys"),
	}

	client.initHTTPClient()
	client.saveConfig()

	return client
}

// NewTLSClient create a basic client with TLS configuration
func NewTLSClient(machines []string, cert, key, caCert string) (*Client, error) {
	// overwrite the default machine to use https
	if len(machines) == 0 {
		machines = []string{"https://127.0.0.1:4001"}
	}

	config := Config{
		// default timeout is one second
		Timeout: time.Second,
		// default consistency level is STRONG
		Consistency: STRONG_CONSISTENCY,
		CertFile:    cert,
		KeyFile:     key,
		CaCertFile:  make([]string, 0),
	}

	client := &Client{
		cluster:   NewCluster(machines),
		config:    config,
		keyPrefix: path.Join(version, "keys"),
	}

	err := client.initHTTPSClient(cert, key)
	if err != nil {
		return nil, err
	}

	err = client.AddRootCA(caCert)

	client.saveConfig()

	return client, nil
}

// NewClientFromFile creates a client from a given file path.
// The given file is expected to use the JSON format.
func NewClientFromFile(fpath string) (*Client, error) {
	fi, err := os.Open(fpath)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := fi.Close(); err != nil {
			panic(err)
		}
	}()

	return NewClientFromReader(fi)
}

// NewClientFromReader creates a Client configured from a given reader.
// The configuration is expected to use the JSON format.
func NewClientFromReader(reader io.Reader) (*Client, error) {
	c := new(Client)

	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(b, c)
	if err != nil {
		return nil, err
	}
	if c.config.CertFile == "" {
		c.initHTTPClient()
	} else {
		err = c.initHTTPSClient(c.config.CertFile, c.config.KeyFile)
	}

	if err != nil {
		return nil, err
	}

	for _, caCert := range c.config.CaCertFile {
		if err := c.AddRootCA(caCert); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Override the Client's HTTP Transport object
func (c *Client) SetTransport(tr *http.Transport) {
	c.httpClient.Transport = tr
}

// SetKeyPrefix changes the key prefix from the default `/v2/keys` to whatever
// is set.
func (c *Client) SetKeyPrefix(prefix string) {
	c.keyPrefix = prefix
}

// initHTTPClient initializes a HTTP client for etcd client
func (c *Client) initHTTPClient() {
	tr := &http.Transport{
		Dial: dialTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	c.httpClient = &http.Client{Transport: tr}
}

// initHTTPClient initializes a HTTPS client for etcd client
func (c *Client) initHTTPSClient(cert, key string) error {
	if cert == "" || key == "" {
		return errors.New("Require both cert and key path")
	}

	tlsCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
		Dial:            dialTimeout,
	}

	c.httpClient = &http.Client{Transport: tr}
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

// AddRootCA adds a root CA cert for the etcd client
func (c *Client) AddRootCA(caCert string) error {
	if c.httpClient == nil {
		return errors.New("Client has not been initialized yet!")
	}

	certBytes, err := ioutil.ReadFile(caCert)
	if err != nil {
		return err
	}

	tr, ok := c.httpClient.Transport.(*http.Transport)

	if !ok {
		panic("AddRootCA(): Transport type assert should not fail")
	}

	if tr.TLSClientConfig.RootCAs == nil {
		caCertPool := x509.NewCertPool()
		ok = caCertPool.AppendCertsFromPEM(certBytes)
		if ok {
			tr.TLSClientConfig.RootCAs = caCertPool
		}
		tr.TLSClientConfig.InsecureSkipVerify = false
	} else {
		ok = tr.TLSClientConfig.RootCAs.AppendCertsFromPEM(certBytes)
	}

	if !ok {
		err = errors.New("Unable to load caCert")
	}

	c.config.CaCertFile = append(c.config.CaCertFile, caCert)
	c.saveConfig()

	return err
}

// SetCluster updates cluster information using the given machine list.
func (c *Client) SetCluster(machines []string) bool {
	success := c.internalSyncCluster(machines)
	return success
}

func (c *Client) GetCluster() []string {
	return c.cluster.Machines
}

// SyncCluster updates the cluster information using the internal machine list.
func (c *Client) SyncCluster() bool {
	return c.internalSyncCluster(c.cluster.Machines)
}

// internalSyncCluster syncs cluster information using the given machine list.
func (c *Client) internalSyncCluster(machines []string) bool {
	for _, machine := range machines {
		httpPath := c.createHttpPath(machine, path.Join(version, "machines"))
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
			c.cluster.updateFromStr(string(b))

			// update leader
			// the first one in the machine list is the leader
			c.cluster.switchLeader(0)

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
	u, err := url.Parse(serverName)
	if err != nil {
		panic(err)
	}

	u.Path = path.Join(u.Path, _path)

	if u.Scheme == "" {
		u.Scheme = "http"
	}
	return u.String()
}

// Dial with timeout.
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, time.Second)
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

// MarshalJSON implements the Marshaller interface
// as defined by the standard JSON package.
func (c *Client) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(struct {
		Config  Config   `json:"config"`
		Cluster *Cluster `json:"cluster"`
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
		Config  Config   `json: "config"`
		Cluster *Cluster `json: "cluster"`
	}{}
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}

	c.cluster = temp.Cluster
	c.config = temp.Config
	return nil
}
