// Copyright 2016 The etcd Authors
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

package clientv3

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"

	"golang.org/x/net/context"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var (
	ErrNoAvailableEndpoints = errors.New("etcdclient: no available endpoints")

	// minConnRetryWait is the minimum time between reconnects to avoid flooding
	minConnRetryWait = time.Second
)

// Client provides and manages an etcd v3 client session.
type Client struct {
	Cluster
	KV
	Lease
	Watcher
	Auth
	Maintenance

	conn   *grpc.ClientConn
	cfg    Config
	creds  *credentials.TransportAuthenticator
	mu     sync.RWMutex // protects connection selection and error list
	errors []error      // errors passed to retryConnection

	ctx    context.Context
	cancel context.CancelFunc

	// fields below are managed by connMonitor

	// reconnc accepts writes which signal the client should reconnect
	reconnc chan error
	// newconnc is closed on successful connect and set to a fresh channel
	newconnc    chan struct{}
	lastConnErr error

	// Username is a username for authentication
	Username string
	// Password is a password for authentication
	Password string
}

// New creates a new etcdv3 client from a given configuration.
func New(cfg Config) (*Client, error) {
	if cfg.retryDialer == nil {
		cfg.retryDialer = dialEndpointList
	}
	if len(cfg.Endpoints) == 0 {
		return nil, ErrNoAvailableEndpoints
	}

	return newClient(&cfg)
}

// NewFromURL creates a new etcdv3 client from a URL.
func NewFromURL(url string) (*Client, error) {
	return New(Config{Endpoints: []string{url}})
}

// NewFromConfigFile creates a new etcdv3 client from a configuration file.
func NewFromConfigFile(path string) (*Client, error) {
	cfg, err := configFromFile(path)
	if err != nil {
		return nil, err
	}
	return New(*cfg)
}

// Close shuts down the client's etcd connections.
func (c *Client) Close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// acquire the cancel
	if c.cancel == nil {
		// already canceled
		if c.lastConnErr != c.ctx.Err() {
			err = c.lastConnErr
		}
		return
	}
	cancel := c.cancel
	c.cancel = nil
	c.mu.Unlock()

	// close watcher and lease before terminating connection
	// so they don't retry on a closed client
	c.Watcher.Close()
	c.Lease.Close()

	// cancel reconnection loop
	cancel()
	c.mu.Lock()
	connc := c.newconnc
	c.mu.Unlock()
	// connc on cancel() is left closed
	<-connc
	c.mu.Lock()
	if c.lastConnErr != c.ctx.Err() {
		err = c.lastConnErr
	}
	return
}

// Ctx is a context for "out of band" messages (e.g., for sending
// "clean up" message when another context is canceled). It is
// canceled on client Close().
func (c *Client) Ctx() context.Context { return c.ctx }

// Endpoints lists the registered endpoints for the client.
func (c *Client) Endpoints() []string { return c.cfg.Endpoints }

// Errors returns all errors that have been observed since called last.
func (c *Client) Errors() (errs []error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	errs = c.errors
	c.errors = nil
	return errs
}

type authTokenCredential struct {
	token string
}

func (cred authTokenCredential) RequireTransportSecurity() bool {
	return false
}

func (cred authTokenCredential) GetRequestMetadata(ctx context.Context, s ...string) (map[string]string, error) {
	return map[string]string{
		"token": cred.token,
	}, nil
}

// Dial establishes a connection for a given endpoint using the client's config
func (c *Client) Dial(endpoint string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTimeout(c.cfg.DialTimeout),
	}

	proto := "tcp"
	creds := c.creds
	if url, uerr := url.Parse(endpoint); uerr == nil && strings.Contains(endpoint, "://") {
		switch url.Scheme {
		case "unix":
			proto = "unix"
		case "http":
			creds = nil
		case "https":
			if creds == nil {
				tlsconfig := &tls.Config{InsecureSkipVerify: true}
				emptyCreds := credentials.NewTLS(tlsconfig)
				creds = &emptyCreds
			}
		default:
			return nil, fmt.Errorf("unknown scheme %q for %q", url.Scheme, endpoint)
		}
		// strip scheme:// prefix since grpc dials by host
		endpoint = url.Host
	}
	f := func(a string, t time.Duration) (net.Conn, error) {
		select {
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		default:
		}
		return net.DialTimeout(proto, a, t)
	}
	opts = append(opts, grpc.WithDialer(f))

	if creds != nil {
		opts = append(opts, grpc.WithTransportCredentials(*creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	if c.Username != "" && c.Password != "" {
		auth, err := newAuthenticator(endpoint, opts)
		if err != nil {
			return nil, err
		}
		defer auth.close()

		resp, err := auth.authenticate(c.ctx, c.Username, c.Password)
		if err != nil {
			return nil, err
		}

		opts = append(opts, grpc.WithPerRPCCredentials(authTokenCredential{token: resp.Token}))
	}

	conn, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// WithRequireLeader requires client requests to only succeed
// when the cluster has a leader.
func WithRequireLeader(ctx context.Context) context.Context {
	md := metadata.Pairs(rpctypes.MetadataRequireLeaderKey, rpctypes.MetadataHasLeader)
	return metadata.NewContext(ctx, md)
}

func newClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = &Config{retryDialer: dialEndpointList}
	}
	var creds *credentials.TransportAuthenticator
	if cfg.TLS != nil {
		c := credentials.NewTLS(cfg.TLS)
		creds = &c
	}

	// use a temporary skeleton client to bootstrap first connection
	ctx, cancel := context.WithCancel(context.TODO())
	conn, err := cfg.retryDialer(&Client{cfg: *cfg, creds: creds, ctx: ctx, Username: cfg.Username, Password: cfg.Password})
	if err != nil {
		return nil, err
	}
	client := &Client{
		conn:     conn,
		cfg:      *cfg,
		creds:    creds,
		ctx:      ctx,
		cancel:   cancel,
		reconnc:  make(chan error, 1),
		newconnc: make(chan struct{}),
	}

	if cfg.Username != "" && cfg.Password != "" {
		client.Username = cfg.Username
		client.Password = cfg.Password
	}

	go client.connMonitor()

	client.Cluster = NewCluster(client)
	client.KV = NewKV(client)
	client.Lease = NewLease(client)
	client.Watcher = NewWatcher(client)
	client.Auth = NewAuth(client)
	client.Maintenance = NewMaintenance(client)
	if cfg.Logger != nil {
		logger.Set(cfg.Logger)
	} else {
		// disable client side grpc by default
		logger.Set(log.New(ioutil.Discard, "", 0))
	}

	return client, nil
}

// ActiveConnection returns the current in-use connection
func (c *Client) ActiveConnection() *grpc.ClientConn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn == nil {
		panic("trying to return nil active connection")
	}
	return c.conn
}

// retryConnection establishes a new connection
func (c *Client) retryConnection(err error) {
	oldconn := c.conn

	// return holding lock so old connection can be cleaned up in this defer
	defer func() {
		if oldconn != nil {
			oldconn.Close()
			if st, _ := oldconn.State(); st != grpc.Shutdown {
				// wait so grpc doesn't leak sleeping goroutines
				oldconn.WaitForStateChange(context.Background(), st)
			}
		}
		c.mu.Unlock()
	}()

	c.mu.Lock()
	if err != nil {
		c.errors = append(c.errors, err)
	}
	if c.cancel == nil {
		// client has called Close() so don't try to dial out
		return
	}
	c.mu.Unlock()

	nc, dialErr := c.cfg.retryDialer(c)

	c.mu.Lock()
	if nc != nil {
		c.conn = nc
	}
	if dialErr != nil {
		c.errors = append(c.errors, dialErr)
	}
	c.lastConnErr = dialErr
}

// connStartRetry schedules a reconnect if one is not already running
func (c *Client) connStartRetry(err error) {
	c.mu.Lock()
	ch := c.reconnc
	defer c.mu.Unlock()
	select {
	case ch <- err:
	default:
	}
}

// connWait waits for a reconnect to be processed
func (c *Client) connWait(ctx context.Context, err error) (*grpc.ClientConn, error) {
	c.mu.RLock()
	ch := c.newconnc
	c.mu.RUnlock()
	c.connStartRetry(err)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ch:
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cancel == nil {
		return c.conn, rpctypes.ErrConnClosed
	}
	return c.conn, c.lastConnErr
}

// connMonitor monitors the connection and handles retries
func (c *Client) connMonitor() {
	var err error

	defer func() {
		c.retryConnection(c.ctx.Err())
		close(c.newconnc)
	}()

	limiter := rate.NewLimiter(rate.Every(minConnRetryWait), 1)
	for limiter.Wait(c.ctx) == nil {
		select {
		case err = <-c.reconnc:
		case <-c.ctx.Done():
			return
		}
		c.retryConnection(err)
		c.mu.Lock()
		close(c.newconnc)
		c.newconnc = make(chan struct{})
		c.reconnc = make(chan error, 1)
		c.mu.Unlock()
	}
}

// dialEndpointList attempts to connect to each endpoint in order until a
// connection is established.
func dialEndpointList(c *Client) (*grpc.ClientConn, error) {
	var err error
	for _, ep := range c.Endpoints() {
		conn, curErr := c.Dial(ep)
		if curErr != nil {
			err = curErr
		} else {
			return conn, nil
		}
	}
	return nil, err
}

// isHaltErr returns true if the given error and context indicate no forward
// progress can be made, even after reconnecting.
func isHaltErr(ctx context.Context, err error) bool {
	isRPCError := strings.HasPrefix(grpc.ErrorDesc(err), "etcdserver: ")
	return isRPCError || ctx.Err() != nil || err == rpctypes.ErrConnClosed
}
