// Copyright 2017 PingCAP, Inc.
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

package statistics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/ddl/util"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/store/tikv/oracle"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/util/sqlexec"
	log "github.com/sirupsen/logrus"
)

type statsCache map[int64]*Table

// Handle can update stats info periodically.
type Handle struct {
	mu struct {
		sync.Mutex
		ctx sessionctx.Context
		// lastVersion is the latest update version before last lease.
		lastVersion uint64
		// rateMap contains the error rate delta from feedback.
		rateMap errorRateDeltaMap
		// pid2tid is the map from partition ID to table ID.
		pid2tid map[int64]int64
		// schemaVersion is the version of information schema when `pid2tid` is built.
		schemaVersion int64
	}

	restrictedExec sqlexec.RestrictedSQLExecutor

	statsCache atomic.Value
	// ddlEventCh is a channel to notify a ddl operation has happened.
	// It is sent only by owner or the drop stats executor, and read by stats handle.
	ddlEventCh chan *util.Event
	// listHead contains all the stats collector required by session.
	listHead *SessionStatsCollector
	// globalMap contains all the delta map from collectors when we dump them to KV.
	globalMap tableDeltaMap
	// feedback is used to store query feedback info.
	feedback []*QueryFeedback

	Lease time.Duration
}

// Clear the statsCache, only for test.
func (h *Handle) Clear() {
	h.mu.Lock()
	h.statsCache.Store(statsCache{})
	h.mu.lastVersion = 0
	for len(h.ddlEventCh) > 0 {
		<-h.ddlEventCh
	}
	h.feedback = h.feedback[:0]
	h.mu.ctx.GetSessionVars().MaxChunkSize = 1
	h.listHead = &SessionStatsCollector{mapper: make(tableDeltaMap), rateMap: make(errorRateDeltaMap)}
	h.globalMap = make(tableDeltaMap)
	h.mu.rateMap = make(errorRateDeltaMap)
	h.mu.Unlock()
}

// MaxQueryFeedbackCount is the max number of feedback that cache in memory.
var MaxQueryFeedbackCount = 1 << 10

// NewHandle creates a Handle for update stats.
func NewHandle(ctx sessionctx.Context, lease time.Duration) *Handle {
	handle := &Handle{
		ddlEventCh: make(chan *util.Event, 100),
		listHead:   &SessionStatsCollector{mapper: make(tableDeltaMap), rateMap: make(errorRateDeltaMap)},
		globalMap:  make(tableDeltaMap),
		Lease:      lease,
		feedback:   make([]*QueryFeedback, 0, MaxQueryFeedbackCount),
	}
	// It is safe to use it concurrently because the exec won't touch the ctx.
	if exec, ok := ctx.(sqlexec.RestrictedSQLExecutor); ok {
		handle.restrictedExec = exec
	}
	handle.mu.ctx = ctx
	handle.mu.rateMap = make(errorRateDeltaMap)
	handle.statsCache.Store(statsCache{})
	return handle
}

// GetQueryFeedback gets the query feedback. It is only use in test.
func (h *Handle) GetQueryFeedback() []*QueryFeedback {
	defer func() {
		h.feedback = h.feedback[:0]
	}()
	return h.feedback
}

// DurationToTS converts duration to timestamp.
func DurationToTS(d time.Duration) uint64 {
	return oracle.ComposeTS(d.Nanoseconds()/int64(time.Millisecond), 0)
}

// Update reads stats meta from store and updates the stats map.
func (h *Handle) Update(is infoschema.InfoSchema) error {
	lastVersion := h.LastUpdateVersion()
	// We need this because for two tables, the smaller version may write later than the one with larger version.
	// Consider the case that there are two tables A and B, their version and commit time is (A0, A1) and (B0, B1),
	// and A0 < B0 < B1 < A1. We will first read the stats of B, and update the lastVersion to B0, but we cannot read
	// the table stats of A0 if we read stats that greater than lastVersion which is B0.
	// We can read the stats if the diff between commit time and version is less than three lease.
	offset := DurationToTS(3 * h.Lease)
	if lastVersion >= offset {
		lastVersion = lastVersion - offset
	} else {
		lastVersion = 0
	}
	sql := fmt.Sprintf("SELECT version, table_id, modify_count, count from mysql.stats_meta where version > %d order by version", lastVersion)
	rows, _, err := h.restrictedExec.ExecRestrictedSQL(nil, sql)
	if err != nil {
		return errors.Trace(err)
	}

	tables := make([]*Table, 0, len(rows))
	deletedTableIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		version := row.GetUint64(0)
		physicalID := row.GetInt64(1)
		modifyCount := row.GetInt64(2)
		count := row.GetInt64(3)
		lastVersion = version
		h.mu.Lock()
		table, ok := h.getTableByPhysicalID(is, physicalID)
		h.mu.Unlock()
		if !ok {
			log.Debugf("Unknown physical ID %d in stats meta table, maybe it has been dropped", physicalID)
			deletedTableIDs = append(deletedTableIDs, physicalID)
			continue
		}
		tableInfo := table.Meta()
		tbl, err := h.tableStatsFromStorage(tableInfo, physicalID, false)
		// Error is not nil may mean that there are some ddl changes on this table, we will not update it.
		if err != nil {
			log.Debugf("Error occurred when read table stats for table %s. The error message is %s.", tableInfo.Name.O, errors.ErrorStack(err))
			continue
		}
		if tbl == nil {
			deletedTableIDs = append(deletedTableIDs, physicalID)
			continue
		}
		tbl.Version = version
		tbl.Count = count
		tbl.ModifyCount = modifyCount
		tbl.name = getFullTableName(is, tableInfo)
		tables = append(tables, tbl)
	}
	h.mu.Lock()
	h.mu.lastVersion = lastVersion
	h.UpdateTableStats(tables, deletedTableIDs)
	h.mu.Unlock()
	return nil
}

func (h *Handle) getTableByPhysicalID(is infoschema.InfoSchema, physicalID int64) (table.Table, bool) {
	if is.SchemaMetaVersion() != h.mu.schemaVersion {
		h.mu.schemaVersion = is.SchemaMetaVersion()
		h.mu.pid2tid = buildPartitionID2TableID(is)
	}
	if id, ok := h.mu.pid2tid[physicalID]; ok {
		return is.TableByID(id)
	}
	return is.TableByID(physicalID)
}

func buildPartitionID2TableID(is infoschema.InfoSchema) map[int64]int64 {
	mapper := make(map[int64]int64)
	for _, db := range is.AllSchemas() {
		tbls := db.Tables
		for _, tbl := range tbls {
			pi := tbl.GetPartitionInfo()
			if pi == nil {
				continue
			}
			for _, def := range pi.Definitions {
				mapper[def.ID] = tbl.ID
			}
		}
	}
	return mapper
}

// GetTableStats retrieves the statistics table from cache, and the cache will be updated by a goroutine.
func (h *Handle) GetTableStats(tblInfo *model.TableInfo) *Table {
	return h.GetPartitionStats(tblInfo, tblInfo.ID)
}

// GetPartitionStats retrieves the partition stats from cache.
func (h *Handle) GetPartitionStats(tblInfo *model.TableInfo, pid int64) *Table {
	tbl, ok := h.statsCache.Load().(statsCache)[pid]
	if !ok {
		tbl = PseudoTable(tblInfo)
		tbl.PhysicalID = pid
		h.UpdateTableStats([]*Table{tbl}, nil)
		return tbl
	}
	return tbl
}

func (h *Handle) copyFromOldCache() statsCache {
	newCache := statsCache{}
	oldCache := h.statsCache.Load().(statsCache)
	for k, v := range oldCache {
		newCache[k] = v
	}
	return newCache
}

// UpdateTableStats updates the statistics table cache using copy on write.
func (h *Handle) UpdateTableStats(tables []*Table, deletedIDs []int64) {
	newCache := h.copyFromOldCache()
	for _, tbl := range tables {
		id := tbl.PhysicalID
		newCache[id] = tbl
	}
	for _, id := range deletedIDs {
		delete(newCache, id)
	}
	h.statsCache.Store(newCache)
}

// LoadNeededHistograms will load histograms for those needed columns.
func (h *Handle) LoadNeededHistograms() error {
	cols := histogramNeededColumns.allCols()
	for _, col := range cols {
		tbl, ok := h.statsCache.Load().(statsCache)[col.tableID]
		if !ok {
			continue
		}
		tbl = tbl.copy()
		c, ok := tbl.Columns[col.columnID]
		if !ok || c.Len() > 0 {
			histogramNeededColumns.delete(col)
			continue
		}
		hg, err := h.histogramFromStorage(col.tableID, c.ID, &c.Info.FieldType, c.NDV, 0, c.LastUpdateVersion, c.NullCount, c.TotColSize)
		if err != nil {
			return errors.Trace(err)
		}
		cms, err := h.cmSketchFromStorage(col.tableID, 0, col.columnID)
		if err != nil {
			return errors.Trace(err)
		}
		tbl.Columns[c.ID] = &Column{Histogram: *hg, Info: c.Info, CMSketch: cms, Count: int64(hg.totalRowCount()), isHandle: c.isHandle}
		h.UpdateTableStats([]*Table{tbl}, nil)
		histogramNeededColumns.delete(col)
	}
	return nil
}

// LastUpdateVersion gets the last update version.
func (h *Handle) LastUpdateVersion() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.mu.lastVersion
}

// SetLastUpdateVersion sets the last update version.
func (h *Handle) SetLastUpdateVersion(version uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mu.lastVersion = version
}

// FlushStats flushes the cached stats update into store.
func (h *Handle) FlushStats() {
	for len(h.ddlEventCh) > 0 {
		e := <-h.ddlEventCh
		if err := h.HandleDDLEvent(e); err != nil {
			log.Debug("[stats] handle ddl event fail: ", errors.ErrorStack(err))
		}
	}
	if err := h.DumpStatsDeltaToKV(DumpAll); err != nil {
		log.Debug("[stats] dump stats delta fail: ", errors.ErrorStack(err))
	}
	if err := h.DumpStatsFeedbackToKV(); err != nil {
		log.Debug("[stats] dump stats feedback fail: ", errors.ErrorStack(err))
	}
}
