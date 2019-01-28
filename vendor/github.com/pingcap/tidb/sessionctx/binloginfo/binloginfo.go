// Copyright 2016 PingCAP, Inc.
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

package binloginfo

import (
	"io/ioutil"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb-tools/tidb-binlog/node"
	pumpcli "github.com/pingcap/tidb-tools/tidb-binlog/pump_client"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/metrics"
	"github.com/pingcap/tidb/sessionctx"
	binlog "github.com/pingcap/tipb/go-binlog"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func init() {
	grpc.EnableTracing = false
	// don't need output pumps client's log
	pumpcli.Logger.Out = ioutil.Discard
}

// pumpsClient is the client to write binlog, it is opened on server start and never close,
// shared by all sessions.
var pumpsClient *pumpcli.PumpsClient
var pumpsClientLock sync.RWMutex

// BinlogInfo contains binlog data and binlog client.
type BinlogInfo struct {
	Data   *binlog.Binlog
	Client *pumpcli.PumpsClient
}

// GetPumpsClient gets the pumps client instance.
func GetPumpsClient() *pumpcli.PumpsClient {
	pumpsClientLock.RLock()
	client := pumpsClient
	pumpsClientLock.RUnlock()
	return client
}

// SetPumpsClient sets the pumps client instance.
func SetPumpsClient(client *pumpcli.PumpsClient) {
	pumpsClientLock.Lock()
	pumpsClient = client
	pumpsClientLock.Unlock()
}

// GetPrewriteValue gets binlog prewrite value in the context.
func GetPrewriteValue(ctx sessionctx.Context, createIfNotExists bool) *binlog.PrewriteValue {
	vars := ctx.GetSessionVars()
	v, ok := vars.TxnCtx.Binlog.(*binlog.PrewriteValue)
	if !ok && createIfNotExists {
		schemaVer := ctx.GetSessionVars().TxnCtx.SchemaVersion
		v = &binlog.PrewriteValue{SchemaVersion: schemaVer}
		vars.TxnCtx.Binlog = v
	}
	return v
}

var skipBinlog uint32
var ignoreError uint32

// DisableSkipBinlogFlag disable the skipBinlog flag.
func DisableSkipBinlogFlag() {
	atomic.StoreUint32(&skipBinlog, 0)
	log.Warn("[binloginfo] disable the skipBinlog flag")
}

// SetIgnoreError sets the ignoreError flag, this function called when TiDB start
// up and find config.Binlog.IgnoreError is true.
func SetIgnoreError(on bool) {
	if on {
		atomic.StoreUint32(&ignoreError, 1)
	} else {
		atomic.StoreUint32(&ignoreError, 0)
	}
}

// WriteBinlog writes a binlog to Pump.
func (info *BinlogInfo) WriteBinlog(clusterID uint64) error {
	skip := atomic.LoadUint32(&skipBinlog)
	if skip > 0 {
		metrics.CriticalErrorCounter.Add(1)
		return nil
	}

	if info.Client == nil {
		return errors.New("pumps client is nil")
	}

	// it will retry in PumpsClient if write binlog fail.
	err := info.Client.WriteBinlog(info.Data)
	if err != nil {
		log.Errorf("write binlog fail %v", errors.ErrorStack(err))
		if atomic.LoadUint32(&ignoreError) == 1 {
			log.Error("write binlog fail but error ignored")
			metrics.CriticalErrorCounter.Add(1)
			// If error happens once, we'll stop writing binlog.
			atomic.CompareAndSwapUint32(&skipBinlog, skip, skip+1)
			return nil
		}

		if strings.Contains(err.Error(), "received message larger than max") {
			// This kind of error is not critical, return directly.
			return errors.Errorf("binlog data is too large (%s)", err.Error())
		}

		return terror.ErrCritical.GenWithStackByArgs(err)
	}

	return nil
}

// SetDDLBinlog sets DDL binlog in the kv.Transaction.
func SetDDLBinlog(client *pumpcli.PumpsClient, txn kv.Transaction, jobID int64, ddlQuery string) {
	if client == nil {
		return
	}

	ddlQuery = addSpecialComment(ddlQuery)
	info := &BinlogInfo{
		Data: &binlog.Binlog{
			Tp:       binlog.BinlogType_Prewrite,
			DdlJobId: jobID,
			DdlQuery: []byte(ddlQuery),
		},
		Client: client,
	}
	txn.SetOption(kv.BinlogInfo, info)
}

const specialPrefix = `/*!90000 `

func addSpecialComment(ddlQuery string) string {
	if strings.Contains(ddlQuery, specialPrefix) {
		return ddlQuery
	}
	upperQuery := strings.ToUpper(ddlQuery)
	reg, err := regexp.Compile(`SHARD_ROW_ID_BITS\s*=\s*\d+`)
	terror.Log(err)
	loc := reg.FindStringIndex(upperQuery)
	if len(loc) < 2 {
		return ddlQuery
	}
	return ddlQuery[:loc[0]] + specialPrefix + ddlQuery[loc[0]:loc[1]] + ` */` + ddlQuery[loc[1]:]
}

// MockPumpsClient creates a PumpsClient, used for test.
func MockPumpsClient(client binlog.PumpClient) *pumpcli.PumpsClient {
	nodeID := "pump-1"
	pump := &pumpcli.PumpStatus{
		Status: node.Status{
			NodeID: nodeID,
			State:  node.Online,
		},
		IsAvaliable: true,
		Client:      client,
	}

	pumpInfos := &pumpcli.PumpInfos{
		Pumps:            make(map[string]*pumpcli.PumpStatus),
		AvaliablePumps:   make(map[string]*pumpcli.PumpStatus),
		UnAvaliablePumps: make(map[string]*pumpcli.PumpStatus),
	}
	pumpInfos.Pumps[nodeID] = pump
	pumpInfos.AvaliablePumps[nodeID] = pump

	pCli := &pumpcli.PumpsClient{
		ClusterID:          1,
		Pumps:              pumpInfos,
		Selector:           pumpcli.NewSelector(pumpcli.Range),
		RetryTime:          1,
		BinlogWriteTimeout: 15 * time.Second,
	}
	pCli.Selector.SetPumps([]*pumpcli.PumpStatus{pump})

	return pCli
}
