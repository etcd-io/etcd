// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/pingcap/errors"
	pd "github.com/pingcap/pd/client"
	"github.com/pingcap/tidb-tools/pkg/etcd"
	"github.com/pingcap/tidb-tools/pkg/utils"
	"github.com/pingcap/tidb-tools/tidb-binlog/node"
	pb "github.com/pingcap/tipb/go-binlog"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultEtcdTimeout is the default timeout config for etcd.
	DefaultEtcdTimeout = 5 * time.Second

	// DefaultAllRetryTime is the default retry time for all pumps, should greter than RetryTime.
	DefaultAllRetryTime = 20

	// RetryTime is the retry time for each pump.
	RetryTime = 5

	// DefaultBinlogWriteTimeout is the default max time binlog can use to write to pump.
	DefaultBinlogWriteTimeout = 15 * time.Second

	// CheckInterval is the default interval for check unavaliable pumps.
	CheckInterval = 30 * time.Second

	// RetryInterval is the default interval of retrying to write binlog.
	RetryInterval = 100 * time.Millisecond
)

var (
	// Logger is ...
	Logger = log.New()

	// ErrNoAvaliablePump means no avaliable pump to write binlog.
	ErrNoAvaliablePump = errors.New("no avaliable pump to write binlog")
)

// PumpInfos saves pumps' infomations in pumps client.
type PumpInfos struct {
	sync.RWMutex
	// Pumps saves the map of pump's nodeID and pump status.
	Pumps map[string]*PumpStatus

	// AvliablePumps saves the whole avaliable pumps' status.
	AvaliablePumps map[string]*PumpStatus

	// UnAvaliablePumps saves the unAvaliable pumps.
	// And only pump with Online state in this map need check is it avaliable.
	UnAvaliablePumps map[string]*PumpStatus
}

// NewPumpInfos returns a PumpInfos.
func NewPumpInfos() *PumpInfos {
	return &PumpInfos{
		Pumps:            make(map[string]*PumpStatus),
		AvaliablePumps:   make(map[string]*PumpStatus),
		UnAvaliablePumps: make(map[string]*PumpStatus),
	}
}

// PumpsClient is the client of pumps.
type PumpsClient struct {
	ctx context.Context

	cancel context.CancelFunc

	wg sync.WaitGroup

	// ClusterID is the cluster ID of this tidb cluster.
	ClusterID uint64

	// the registry of etcd.
	EtcdRegistry *node.EtcdRegistry

	// Pumps saves the pumps' information.
	Pumps *PumpInfos

	// Selector will select a suitable pump.
	Selector PumpSelector

	// the max retry time if write binlog failed.
	RetryTime int

	// BinlogWriteTimeout is the max time binlog can use to write to pump.
	BinlogWriteTimeout time.Duration

	// Security is the security config
	Security *tls.Config

	// binlog socket file path, for compatible with kafka version pump.
	binlogSocket string
}

// NewPumpsClient returns a PumpsClient.
// TODO: get strategy from etcd, and can update strategy in real-time. Use Range as default now.
func NewPumpsClient(etcdURLs string, timeout time.Duration, securityOpt pd.SecurityOption) (*PumpsClient, error) {
	ectdEndpoints, err := utils.ParseHostPortAddr(etcdURLs)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// get clusterid
	pdCli, err := pd.NewClient(ectdEndpoints, securityOpt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	clusterID := pdCli.GetClusterID(context.Background())
	pdCli.Close()

	security, err := utils.ToTLSConfig(securityOpt.CAPath, securityOpt.CertPath, securityOpt.KeyPath)
	if err != nil {
		return nil, errors.Trace(err)
	}

	cli, err := etcd.NewClientFromCfg(ectdEndpoints, DefaultEtcdTimeout, node.DefaultRootPath, security)
	if err != nil {
		return nil, errors.Trace(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	newPumpsClient := &PumpsClient{
		ctx:                ctx,
		cancel:             cancel,
		ClusterID:          clusterID,
		EtcdRegistry:       node.NewEtcdRegistry(cli, DefaultEtcdTimeout),
		Pumps:              NewPumpInfos(),
		Selector:           NewSelector(Range),
		RetryTime:          DefaultAllRetryTime,
		BinlogWriteTimeout: timeout,
		Security:           security,
	}

	err = newPumpsClient.getPumpStatus(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	newPumpsClient.Selector.SetPumps(copyPumps(newPumpsClient.Pumps.AvaliablePumps))

	newPumpsClient.wg.Add(2)
	go newPumpsClient.watchStatus()
	go newPumpsClient.detect()

	return newPumpsClient, nil
}

// NewLocalPumpsClient returns a PumpsClient, this PumpsClient will write binlog by socket file. For compatible with kafka version pump.
func NewLocalPumpsClient(etcdURLs, binlogSocket string, timeout time.Duration, securityOpt pd.SecurityOption) (*PumpsClient, error) {
	ectdEndpoints, err := utils.ParseHostPortAddr(etcdURLs)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// get clusterid
	pdCli, err := pd.NewClient(ectdEndpoints, securityOpt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	clusterID := pdCli.GetClusterID(context.Background())
	pdCli.Close()

	security, err := utils.ToTLSConfig(securityOpt.CAPath, securityOpt.CertPath, securityOpt.KeyPath)
	if err != nil {
		return nil, errors.Trace(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	newPumpsClient := &PumpsClient{
		ctx:                ctx,
		cancel:             cancel,
		ClusterID:          clusterID,
		Pumps:              NewPumpInfos(),
		Selector:           NewSelector(LocalUnix),
		RetryTime:          DefaultAllRetryTime,
		BinlogWriteTimeout: timeout,
		Security:           security,
		binlogSocket:       binlogSocket,
	}
	newPumpsClient.getLocalPumpStatus(ctx)

	return newPumpsClient, nil
}

// getLocalPumpStatus gets the local pump. For compatible with kafka version tidb-binlog.
func (c *PumpsClient) getLocalPumpStatus(pctx context.Context) {
	nodeStatus := &node.Status{
		NodeID:  localPump,
		Addr:    c.binlogSocket,
		IsAlive: true,
		State:   node.Online,
	}
	c.addPump(NewPumpStatus(nodeStatus, c.Security), true)
}

// getPumpStatus gets all the pumps status in the etcd.
func (c *PumpsClient) getPumpStatus(pctx context.Context) error {
	nodesStatus, err := c.EtcdRegistry.Nodes(pctx, node.NodePrefix[node.PumpNode])
	if err != nil {
		return errors.Trace(err)
	}

	for _, status := range nodesStatus {
		Logger.Debugf("[pumps client] get pump %v from etcd", status)
		c.addPump(NewPumpStatus(status, c.Security), false)
	}

	return nil
}

// WriteBinlog writes binlog to a situable pump.
func (c *PumpsClient) WriteBinlog(binlog *pb.Binlog) error {
	pump := c.Selector.Select(binlog)
	if pump == nil {
		return ErrNoAvaliablePump
	}
	Logger.Debugf("[pumps client] write binlog choose pump %s", pump.NodeID)

	commitData, err := binlog.Marshal()
	if err != nil {
		return errors.Trace(err)
	}
	req := &pb.WriteBinlogReq{ClusterID: c.ClusterID, Payload: commitData}

	retryTime := 0
	startTime := time.Now()
	var resp *pb.WriteBinlogResp

	for {
		if pump == nil {
			return ErrNoAvaliablePump
		}

		resp, err = pump.writeBinlog(req, c.BinlogWriteTimeout)
		if err == nil && resp.Errmsg != "" {
			err = errors.New(resp.Errmsg)
		}
		if err == nil {
			return nil
		}
		Logger.Errorf("[pumps client] write binlog error %v", err)

		if binlog.Tp == pb.BinlogType_Commit {
			// only use one pump to write commit binlog, util write success or blocked for ten minutes.
			if time.Since(startTime) > 10*time.Minute {
				break
			}
		} else {
			if !isRetryableError(err) {
				// this kind of error is not retryable, return directly.
				return err
			}

			// every pump can retry 5 times, if retry 5 times and still failed, set this pump unavaliable, and choose a new pump.
			if (retryTime+1)%RetryTime == 0 {
				c.setPumpAvaliable(pump, false)
				pump = c.Selector.Next(binlog, retryTime/5+1)
				Logger.Debugf("[pumps client] avaliable pumps: %v, write binlog choose pump %v", c.Pumps.AvaliablePumps, pump)
			}

			retryTime++
			if retryTime > c.RetryTime {
				break
			}
		}

		time.Sleep(RetryInterval)
	}

	return err
}

// setPumpAvaliable set pump's isAvaliable, and modify UnAvaliablePumps or AvaliablePumps.
func (c *PumpsClient) setPumpAvaliable(pump *PumpStatus, avaliable bool) {
	pump.IsAvaliable = avaliable
	if pump.IsAvaliable {
		err := pump.createGrpcClient(c.Security)
		if err != nil {
			Logger.Errorf("[pumps client] create grpc client for pump %s failed, error: %v", pump.NodeID, err)
			pump.IsAvaliable = false
			return
		}

		c.Pumps.Lock()
		delete(c.Pumps.UnAvaliablePumps, pump.NodeID)
		if _, ok := c.Pumps.Pumps[pump.NodeID]; ok {
			c.Pumps.AvaliablePumps[pump.NodeID] = pump
		}
		c.Pumps.Unlock()

	} else {
		c.Pumps.Lock()
		delete(c.Pumps.AvaliablePumps, pump.NodeID)
		if _, ok := c.Pumps.Pumps[pump.NodeID]; ok {
			c.Pumps.UnAvaliablePumps[pump.NodeID] = pump
		}
		c.Pumps.Unlock()
	}

	c.Pumps.RLock()
	c.Selector.SetPumps(copyPumps(c.Pumps.AvaliablePumps))
	c.Pumps.RUnlock()
}

// addPump add a new pump.
func (c *PumpsClient) addPump(pump *PumpStatus, updateSelector bool) {
	c.Pumps.Lock()

	if pump.State == node.Online {
		c.Pumps.AvaliablePumps[pump.NodeID] = pump
	} else {
		c.Pumps.UnAvaliablePumps[pump.NodeID] = pump
	}
	c.Pumps.Pumps[pump.NodeID] = pump

	if updateSelector {
		c.Selector.SetPumps(copyPumps(c.Pumps.AvaliablePumps))
	}

	c.Pumps.Unlock()
}

// updatePump update pump's status, and return whether pump's IsAvaliable should be changed.
func (c *PumpsClient) updatePump(status *node.Status) (pump *PumpStatus, avaliableChanged, avaliable bool) {
	var ok bool
	c.Pumps.Lock()
	if pump, ok = c.Pumps.Pumps[status.NodeID]; ok {
		if pump.Status.State != status.State {
			if status.State == node.Online {
				avaliableChanged = true
				avaliable = true
			} else if pump.Status.State == node.Online {
				avaliableChanged = true
				avaliable = false
			}
		}
		pump.Status = *status
	}
	c.Pumps.Unlock()

	return
}

// removePump removes a pump.
func (c *PumpsClient) removePump(nodeID string) {
	c.Pumps.Lock()
	if pump, ok := c.Pumps.Pumps[nodeID]; ok {
		pump.closeGrpcClient()
	}
	delete(c.Pumps.Pumps, nodeID)
	delete(c.Pumps.UnAvaliablePumps, nodeID)
	delete(c.Pumps.AvaliablePumps, nodeID)
	c.Selector.SetPumps(copyPumps(c.Pumps.AvaliablePumps))
	c.Pumps.Unlock()
}

// exist returns true if pumps client has pump matched this nodeID.
func (c *PumpsClient) exist(nodeID string) bool {
	c.Pumps.RLock()
	_, ok := c.Pumps.Pumps[nodeID]
	c.Pumps.RUnlock()
	return ok
}

// watchStatus watchs pump's status in etcd.
func (c *PumpsClient) watchStatus() {
	defer c.wg.Done()
	rootPath := path.Join(node.DefaultRootPath, node.NodePrefix[node.PumpNode])
	rch := c.EtcdRegistry.WatchNode(c.ctx, rootPath)
	for {
		select {
		case <-c.ctx.Done():
			Logger.Info("[pumps client] watch status finished")
			return
		case wresp := <-rch:
			for _, ev := range wresp.Events {
				status := &node.Status{}
				err := json.Unmarshal(ev.Kv.Value, &status)
				if err != nil {
					Logger.Errorf("[pumps client] unmarshal pump status %q failed", ev.Kv.Value)
					continue
				}

				switch ev.Type {
				case mvccpb.PUT:
					if !c.exist(status.NodeID) {
						Logger.Infof("[pumps client] find a new pump %s", status.NodeID)
						c.addPump(NewPumpStatus(status, c.Security), true)
						continue
					}

					pump, avaliableChanged, avaliable := c.updatePump(status)
					if avaliableChanged {
						Logger.Infof("[pumps client] pump %s's state is changed to %s", pump.Status.NodeID, status.State)
						c.setPumpAvaliable(pump, avaliable)
					}

				case mvccpb.DELETE:
					// now will not delete pump node in fact, just for compatibility.
					nodeID := node.AnalyzeNodeID(string(ev.Kv.Key))
					Logger.Infof("[pumps client] remove pump %s", nodeID)
					c.removePump(nodeID)
				}
			}
		}
	}
}

// detect send detect binlog to pumps with online state in UnAvaliablePumps,
func (c *PumpsClient) detect() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			Logger.Infof("[pumps client] heartbeat finished")
			return
		default:
			// send detect binlog to pump, if this pump can return response without error
			// means this pump is avaliable.
			needCheckPumps := make([]*PumpStatus, 0, len(c.Pumps.UnAvaliablePumps))
			checkPassPumps := make([]*PumpStatus, 0, 1)
			req := &pb.WriteBinlogReq{ClusterID: c.ClusterID, Payload: nil}
			c.Pumps.RLock()
			for _, pump := range c.Pumps.UnAvaliablePumps {
				if pump.Status.State == node.Online {
					needCheckPumps = append(needCheckPumps, pump)
				}
			}
			c.Pumps.RUnlock()

			for _, pump := range needCheckPumps {
				err := pump.createGrpcClient(c.Security)
				if err != nil {
					Logger.Errorf("[pumps client] create grpc client for pump %s failed, error %v", pump.NodeID, errors.Trace(err))
					continue
				}
				if pump.Client == nil {
					continue
				}

				_, err = pump.writeBinlog(req, c.BinlogWriteTimeout)
				if err == nil {
					checkPassPumps = append(checkPassPumps, pump)
				} else {
					Logger.Errorf("[pumps client] write detect binlog to pump %s error %v", pump.NodeID, err)
				}
			}

			for _, pump := range checkPassPumps {
				c.setPumpAvaliable(pump, true)
			}

			time.Sleep(CheckInterval)
		}
	}
}

// Close closes the PumpsClient.
func (c *PumpsClient) Close() {
	Logger.Infof("[pumps client] is closing")
	c.cancel()
	c.wg.Wait()
	Logger.Infof("[pumps client] is closed")
}

func isRetryableError(err error) bool {
	// ResourceExhausted is a error code in grpc.
	// ResourceExhausted indicates some resource has been exhausted, perhaps
	// a per-user quota, or perhaps the entire file system is out of space.
	// https://github.com/grpc/grpc-go/blob/9cc4fdbde2304827ffdbc7896f49db40c5536600/codes/codes.go#L76
	if strings.Contains(err.Error(), "ResourceExhausted") {
		return false
	}

	return true
}

func copyPumps(pumps map[string]*PumpStatus) []*PumpStatus {
	ps := make([]*PumpStatus, 0, len(pumps))
	for _, pump := range pumps {
		ps = append(ps, pump)
	}

	return ps
}
