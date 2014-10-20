/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package client

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnavailable = errors.New("client: no available etcd endpoints")
	ErrNoLeader    = errors.New("client: no leader")
	ErrKeyNoExist  = errors.New("client: key does not exist")
	ErrKeyExists   = errors.New("client: key already exists")
)

type Client interface {
	Create(key, value string, ttl time.Duration) (*Response, error)
	Get(key string) (*Response, error)
	Watch(key string, idx uint64) Watcher
	RecursiveWatch(key string, idx uint64) Watcher
}

type Watcher interface {
	Next() (*Response, error)
}

type Response struct {
	Action   string `json:"action"`
	Node     *Node  `json:"node"`
	PrevNode *Node  `json:"prevNode"`
}

type Nodes []*Node
type Node struct {
	Key           string `json:"key"`
	Value         string `json:"value"`
	Nodes         Nodes  `json:"nodes"`
	ModifiedIndex uint64 `json:"modifiedIndex"`
	CreatedIndex  uint64 `json:"createdIndex"`
}

func (n *Node) String() string {
	return fmt.Sprintf("{Key: %s, CreatedIndex: %d, ModifiedIndex: %d}", n.Key, n.CreatedIndex, n.ModifiedIndex)
}
