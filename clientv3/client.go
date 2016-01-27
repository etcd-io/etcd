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
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type Client struct {
	// KV is the keyvalue API for the client's connection.
	KV pb.KVClient
	// Lease is the lease API for the client's connection.
	Lease pb.LeaseClient
	// Watch is the watch API for the client's connection.
	Watch pb.WatchClient
	// Cluster is the cluster API for the client's connection.
	Cluster pb.ClusterClient

	conn *grpc.ClientConn
	cfg  Config
}

type Config struct {
	// Endpoints is a list of URLs
	Endpoints []string

	// TODO TLS options
}

// New creates a new etcdv3 client from a given configuration.
func New(cfg Config) (*Client, error) {
	conn, err := cfg.dialEndpoints()
	if err != nil {
		return nil, err
	}
	client := newClient(conn)
	client.cfg = cfg
	return client, nil
}

// NewFromURL creates a new etcdv3 client from a URL.
func NewFromURL(url string) (*Client, error) {
	return New(Config{Endpoints: []string{url}})
}

// NewFromConn creates a new etcdv3 client from an established grpc Connection.
func NewFromConn(conn *grpc.ClientConn) *Client {
	return newClient(conn)
}

// Clone creates a copy of client with the old connection and new API clients.
func (c *Client) Clone() *Client {
	cl := newClient(c.conn)
	cl.cfg = c.cfg
	return cl
}

// Close shuts down the client's etcd connections.
func (c *Client) Close() error {
	return c.conn.Close()
}

func newClient(conn *grpc.ClientConn) *Client {
	return &Client{
		KV:      pb.NewKVClient(conn),
		Lease:   pb.NewLeaseClient(conn),
		Watch:   pb.NewWatchClient(conn),
		Cluster: pb.NewClusterClient(conn),
		conn:    conn,
	}
}

func (cfg *Config) dialEndpoints() (*grpc.ClientConn, error) {
	var err error
	for _, ep := range cfg.Endpoints {
		// TODO: enable grpc.WithTransportCredentials(creds)
		conn, curErr := grpc.Dial(ep, grpc.WithInsecure())
		if err != nil {
			err = curErr
		} else {
			return conn, nil
		}
	}
	return nil, err
}
