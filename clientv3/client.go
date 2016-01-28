// Copyright 2016 CoreOS, Inc.
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
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc/codes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

// Client provides and manages an etcd v3 client session.
type Client struct {
	// KV is the keyvalue API for the client's connection.
	KV pb.KVClient
	// Lease is the lease API for the client's connection.
	Lease pb.LeaseClient
	// Watch is the watch API for the client's connection.
	Watch pb.WatchClient
	// Cluster is the cluster API for the client's connection.
	Cluster pb.ClusterClient

	conn   *grpc.ClientConn
	cfg    Config
	mu     sync.RWMutex // protects connection selection and error list
	errors []error      // errors passed to retryConnection
}

// EndpointDialer is a policy for choosing which endpoint to dial next
type EndpointDialer func(*Client) (*grpc.ClientConn, error)

type Config struct {
	// Endpoints is a list of URLs
	Endpoints []string

	// RetryDialer chooses the next endpoint to use
	RetryDialer EndpointDialer

	// DialTimeout is the timeout for failing to establish a connection.
	DialTimeout time.Duration

	// TODO TLS options
}

// New creates a new etcdv3 client from a given configuration.
func New(cfg Config) (*Client, error) {
	if cfg.RetryDialer == nil {
		cfg.RetryDialer = dialEndpointList
	}
	// use a temporary skeleton client to bootstrap first connection
	conn, err := cfg.RetryDialer(&Client{cfg: cfg})
	if err != nil {
		return nil, err
	}
	return newClient(conn, &cfg), nil
}

// NewFromURL creates a new etcdv3 client from a URL.
func NewFromURL(url string) (*Client, error) {
	return New(Config{Endpoints: []string{url}})
}

// NewFromConn creates a new etcdv3 client from an established grpc Connection.
func NewFromConn(conn *grpc.ClientConn) *Client { return newClient(conn, nil) }

// Clone creates a copy of client with the old connection and new API clients.
func (c *Client) Clone() *Client { return newClient(c.conn, &c.cfg) }

// Close shuts down the client's etcd connections.
func (c *Client) Close() error {
	return c.conn.Close()
}

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

// Dial establishes a connection for a given endpoint using the client's config
func (c *Client) Dial(endpoint string) (*grpc.ClientConn, error) {
	// TODO: enable grpc.WithTransportCredentials(creds)
	conn, err := grpc.Dial(
		endpoint,
		grpc.WithBlock(),
		grpc.WithTimeout(c.cfg.DialTimeout),
		grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func newClient(conn *grpc.ClientConn, cfg *Config) *Client {
	if cfg == nil {
		cfg = &Config{RetryDialer: dialEndpointList}
	}
	return &Client{
		KV:      pb.NewKVClient(conn),
		Lease:   pb.NewLeaseClient(conn),
		Watch:   pb.NewWatchClient(conn),
		Cluster: pb.NewClusterClient(conn),
		conn:    conn,
		cfg:     *cfg,
	}
}

// activeConnection returns the current in-use connection
func (c *Client) activeConnection() *grpc.ClientConn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// refreshConnection establishes a new connection
func (c *Client) retryConnection(oldConn *grpc.ClientConn, err error) (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.errors = append(c.errors, err)
	}
	if oldConn != c.conn {
		// conn has already been updated
		return c.conn, nil
	}
	conn, dialErr := c.cfg.RetryDialer(c)
	if dialErr != nil {
		c.errors = append(c.errors, dialErr)
		return nil, dialErr
	}
	c.conn = conn
	return c.conn, nil
}

// dialEndpoints attempts to connect to each endpoint in order until a
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

func isRPCError(err error) bool {
	return grpc.Code(err) != codes.Unknown
}
