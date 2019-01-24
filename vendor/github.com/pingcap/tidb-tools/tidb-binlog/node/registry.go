package node

import (
	"context"
	"encoding/json"
	"path"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb-tools/pkg/etcd"
	log "github.com/sirupsen/logrus"
)

// EtcdRegistry wraps the reactions with etcd
type EtcdRegistry struct {
	client     *etcd.Client
	reqTimeout time.Duration
}

// NewEtcdRegistry returns an EtcdRegistry client
func NewEtcdRegistry(cli *etcd.Client, reqTimeout time.Duration) *EtcdRegistry {
	return &EtcdRegistry{
		client:     cli,
		reqTimeout: reqTimeout,
	}
}

// Close closes the etcd client
func (r *EtcdRegistry) Close() error {
	err := r.client.Close()
	return errors.Trace(err)
}

func (r *EtcdRegistry) prefixed(p ...string) string {
	return path.Join(p...)
}

// Node returns the nodeStatus that matchs nodeID in the etcd
func (r *EtcdRegistry) Node(pctx context.Context, prefix, nodeID string) (*Status, error) {
	ctx, cancel := context.WithTimeout(pctx, r.reqTimeout)
	defer cancel()

	data, err := r.client.Get(ctx, r.prefixed(prefix, nodeID))
	if err != nil {
		return nil, errors.Trace(err)
	}

	status := &Status{}
	if err = json.Unmarshal(data, &status); err != nil {
		return nil, errors.Annotatef(err, "Invalid nodeID(%s)", nodeID)
	}
	return status, nil
}

// Nodes retruns all the nodeStatuses in the etcd
func (r *EtcdRegistry) Nodes(pctx context.Context, prefix string) ([]*Status, error) {
	ctx, cancel := context.WithTimeout(pctx, r.reqTimeout)
	defer cancel()

	resp, err := r.client.List(ctx, r.prefixed(prefix))
	if err != nil {
		return nil, errors.Trace(err)
	}
	status, err := NodesStatusFromEtcdNode(resp)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return status, nil
}

// UpdateNode update the node information.
func (r *EtcdRegistry) UpdateNode(pctx context.Context, prefix string, status *Status) error {
	ctx, cancel := context.WithTimeout(pctx, r.reqTimeout)
	defer cancel()

	if exists, err := r.checkNodeExists(ctx, prefix, status.NodeID); err != nil {
		return errors.Trace(err)
	} else if !exists {
		// not found then create a new node
		log.Infof("node %s dosen't exist, will create one", status.NodeID)
		return r.createNode(ctx, prefix, status)
	} else {
		// found it, update status infomation of the node
		return r.updateNode(ctx, prefix, status)
	}
}

func (r *EtcdRegistry) checkNodeExists(ctx context.Context, prefix, nodeID string) (bool, error) {
	_, err := r.client.Get(ctx, r.prefixed(prefix, nodeID))
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Trace(err)
	}
	return true, nil
}

func (r *EtcdRegistry) updateNode(ctx context.Context, prefix string, status *Status) error {
	objstr, err := json.Marshal(status)
	if err != nil {
		return errors.Annotatef(err, "error marshal NodeStatus(%v)", status)
	}
	key := r.prefixed(prefix, status.NodeID)
	err = r.client.Update(ctx, key, string(objstr), 0)
	return errors.Trace(err)
}

func (r *EtcdRegistry) createNode(ctx context.Context, prefix string, status *Status) error {
	objstr, err := json.Marshal(status)
	if err != nil {
		return errors.Annotatef(err, "error marshal NodeStatus(%v)", status)
	}
	key := r.prefixed(prefix, status.NodeID)
	err = r.client.Create(ctx, key, string(objstr), nil)
	return errors.Trace(err)
}

// WatchNode watchs node's event
func (r *EtcdRegistry) WatchNode(pctx context.Context, prefix string) clientv3.WatchChan {
	return r.client.Watch(pctx, prefix)
}

func nodeStatusFromEtcdNode(id string, node *etcd.Node) (*Status, error) {
	status := &Status{}

	if err := json.Unmarshal(node.Value, &status); err != nil {
		return nil, errors.Annotatef(err, "error unmarshal NodeStatus with nodeID(%s), node value(%s)", id, node.Value)
	}

	return status, nil
}

// NodesStatusFromEtcdNode returns nodes' status under root node.
func NodesStatusFromEtcdNode(root *etcd.Node) ([]*Status, error) {
	var statuses []*Status
	for id, n := range root.Childs {
		status, err := nodeStatusFromEtcdNode(id, n)
		if err != nil {
			return nil, err
		}
		if status == nil {
			continue
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// AnalyzeNodeID returns nodeID by analyze key path.
func AnalyzeNodeID(key string) string {
	// the key looks like: /tidb-binlog/v1/pumps/nodeID, or /tidb-binlog/pumps/nodeID for old binlog version.
	paths := strings.Split(key, "/")
	nodeIDOffset := 3

	if len(paths) >= 2 {
		// version string start with 'v'
		if !strings.HasPrefix(paths[1], "v") {
			nodeIDOffset = 2
		}
	} else {
		log.Errorf("can't get nodeID or node type from key %s", key)
		return ""
	}

	if len(paths) < nodeIDOffset+1 {
		log.Errorf("can't get nodeID or node type from key %s", key)
		return ""
	}

	return paths[nodeIDOffset]
}
