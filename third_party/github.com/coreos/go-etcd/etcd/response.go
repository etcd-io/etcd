package etcd

import (
	"encoding/json"
	"net/http"
	"time"
)

const (
	rawResponse = iota
	normalResponse
)

type responseType int

type RawResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func (rr *RawResponse) toResponse() (*Response, error) {
	if rr.StatusCode == http.StatusBadRequest {
		return nil, handleError(rr.Body)
	}

	resp := new(Response)

	err := json.Unmarshal(rr.Body, resp)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

type Response struct {
	Action string `json:"action"`
	Node   *Node  `json:"node,omitempty"`
}

type Node struct {
	Key           string     `json:"key, omitempty"`
	PrevValue     string     `json:"prevValue,omitempty"`
	Value         string     `json:"value,omitempty"`
	Dir           bool       `json:"dir,omitempty"`
	Expiration    *time.Time `json:"expiration,omitempty"`
	TTL           int64      `json:"ttl,omitempty"`
	Nodes         Nodes      `json:"nodes,omitempty"`
	ModifiedIndex uint64     `json:"modifiedIndex,omitempty"`
	CreatedIndex  uint64     `json:"createdIndex,omitempty"`
}

type Nodes []Node

// interfaces for sorting
func (ns Nodes) Len() int {
	return len(ns)
}

func (ns Nodes) Less(i, j int) bool {
	return ns[i].Key < ns[j].Key
}

func (ns Nodes) Swap(i, j int) {
	ns[i], ns[j] = ns[j], ns[i]
}
