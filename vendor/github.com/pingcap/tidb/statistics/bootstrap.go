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

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/sqlexec"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

func (h *Handle) initStatsMeta4Chunk(is infoschema.InfoSchema, tables statsCache, iter *chunk.Iterator4Chunk) {
	for row := iter.Begin(); row != iter.End(); row = iter.Next() {
		physicalID := row.GetInt64(1)
		table, ok := h.getTableByPhysicalID(is, physicalID)
		if !ok {
			log.Debugf("Unknown physical ID %d in stats meta table, maybe it has been dropped", physicalID)
			continue
		}
		tableInfo := table.Meta()
		newHistColl := HistColl{
			PhysicalID:     physicalID,
			HavePhysicalID: true,
			Count:          row.GetInt64(3),
			ModifyCount:    row.GetInt64(2),
			Columns:        make(map[int64]*Column, len(tableInfo.Columns)),
			Indices:        make(map[int64]*Index, len(tableInfo.Indices)),
		}
		tbl := &Table{
			HistColl: newHistColl,
			Version:  row.GetUint64(0),
			name:     getFullTableName(is, tableInfo),
		}
		tables[physicalID] = tbl
	}
}

func (h *Handle) initStatsMeta(is infoschema.InfoSchema) (statsCache, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sql := "select HIGH_PRIORITY version, table_id, modify_count, count from mysql.stats_meta"
	rc, err := h.mu.ctx.(sqlexec.SQLExecutor).Execute(context.TODO(), sql)
	if len(rc) > 0 {
		defer terror.Call(rc[0].Close)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	tables := statsCache{}
	chk := rc[0].NewChunk()
	iter := chunk.NewIterator4Chunk(chk)
	for {
		err := rc[0].Next(context.TODO(), chk)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if chk.NumRows() == 0 {
			break
		}
		h.initStatsMeta4Chunk(is, tables, iter)
	}
	return tables, nil
}

func (h *Handle) initStatsHistograms4Chunk(is infoschema.InfoSchema, tables statsCache, iter *chunk.Iterator4Chunk) {
	for row := iter.Begin(); row != iter.End(); row = iter.Next() {
		table, ok := tables[row.GetInt64(0)]
		if !ok {
			continue
		}
		id, ndv, nullCount, version, totColSize := row.GetInt64(2), row.GetInt64(3), row.GetInt64(5), row.GetUint64(4), row.GetInt64(7)
		tbl, _ := h.getTableByPhysicalID(is, table.PhysicalID)
		if row.GetInt64(1) > 0 {
			var idxInfo *model.IndexInfo
			for _, idx := range tbl.Meta().Indices {
				if idx.ID == id {
					idxInfo = idx
					break
				}
			}
			if idxInfo == nil {
				continue
			}
			cms, err := decodeCMSketch(row.GetBytes(6))
			if err != nil {
				cms = nil
				terror.Log(errors.Trace(err))
			}
			hist := NewHistogram(id, ndv, nullCount, version, types.NewFieldType(mysql.TypeBlob), chunk.InitialCapacity, 0)
			table.Indices[hist.ID] = &Index{Histogram: *hist, CMSketch: cms, Info: idxInfo, statsVer: row.GetInt64(8)}
		} else {
			var colInfo *model.ColumnInfo
			for _, col := range tbl.Meta().Columns {
				if col.ID == id {
					colInfo = col
					break
				}
			}
			if colInfo == nil {
				continue
			}
			hist := NewHistogram(id, ndv, nullCount, version, &colInfo.FieldType, 0, totColSize)
			table.Columns[hist.ID] = &Column{Histogram: *hist, Info: colInfo, Count: nullCount, isHandle: tbl.Meta().PKIsHandle && mysql.HasPriKeyFlag(colInfo.Flag)}
		}
	}
}

func (h *Handle) initStatsHistograms(is infoschema.InfoSchema, tables statsCache) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	sql := "select HIGH_PRIORITY table_id, is_index, hist_id, distinct_count, version, null_count, cm_sketch, tot_col_size, stats_ver from mysql.stats_histograms"
	rc, err := h.mu.ctx.(sqlexec.SQLExecutor).Execute(context.TODO(), sql)
	if len(rc) > 0 {
		defer terror.Call(rc[0].Close)
	}
	if err != nil {
		return errors.Trace(err)
	}
	chk := rc[0].NewChunk()
	iter := chunk.NewIterator4Chunk(chk)
	for {
		err := rc[0].Next(context.TODO(), chk)
		if err != nil {
			return errors.Trace(err)
		}
		if chk.NumRows() == 0 {
			break
		}
		h.initStatsHistograms4Chunk(is, tables, iter)
	}
	return nil
}

func initStatsBuckets4Chunk(ctx sessionctx.Context, tables statsCache, iter *chunk.Iterator4Chunk) {
	for row := iter.Begin(); row != iter.End(); row = iter.Next() {
		tableID, isIndex, histID := row.GetInt64(0), row.GetInt64(1), row.GetInt64(2)
		table, ok := tables[tableID]
		if !ok {
			continue
		}
		var lower, upper types.Datum
		var hist *Histogram
		if isIndex > 0 {
			index, ok := table.Indices[histID]
			if !ok {
				continue
			}
			hist = &index.Histogram
			lower, upper = types.NewBytesDatum(row.GetBytes(5)), types.NewBytesDatum(row.GetBytes(6))
		} else {
			column, ok := table.Columns[histID]
			if !ok {
				continue
			}
			column.Count += row.GetInt64(3)
			if !mysql.HasPriKeyFlag(column.Info.Flag) {
				continue
			}
			hist = &column.Histogram
			d := types.NewBytesDatum(row.GetBytes(5))
			var err error
			lower, err = d.ConvertTo(ctx.GetSessionVars().StmtCtx, &column.Info.FieldType)
			if err != nil {
				log.Debugf("decode bucket lower bound failed: %s", errors.ErrorStack(err))
				delete(table.Columns, histID)
				continue
			}
			d = types.NewBytesDatum(row.GetBytes(6))
			upper, err = d.ConvertTo(ctx.GetSessionVars().StmtCtx, &column.Info.FieldType)
			if err != nil {
				log.Debugf("decode bucket upper bound failed: %s", errors.ErrorStack(err))
				delete(table.Columns, histID)
				continue
			}
		}
		hist.AppendBucket(&lower, &upper, row.GetInt64(3), row.GetInt64(4))
	}
}

func (h *Handle) initStatsBuckets(tables statsCache) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	sql := "select HIGH_PRIORITY table_id, is_index, hist_id, count, repeats, lower_bound, upper_bound from mysql.stats_buckets order by table_id, is_index, hist_id, bucket_id"
	rc, err := h.mu.ctx.(sqlexec.SQLExecutor).Execute(context.TODO(), sql)
	if len(rc) > 0 {
		defer terror.Call(rc[0].Close)
	}
	if err != nil {
		return errors.Trace(err)
	}
	chk := rc[0].NewChunk()
	iter := chunk.NewIterator4Chunk(chk)
	for {
		err := rc[0].Next(context.TODO(), chk)
		if err != nil {
			return errors.Trace(err)
		}
		if chk.NumRows() == 0 {
			break
		}
		initStatsBuckets4Chunk(h.mu.ctx, tables, iter)
	}
	for _, table := range tables {
		if h.mu.lastVersion < table.Version {
			h.mu.lastVersion = table.Version
		}
		for _, idx := range table.Indices {
			for i := 1; i < idx.Len(); i++ {
				idx.Buckets[i].Count += idx.Buckets[i-1].Count
			}
			idx.PreCalculateScalar()
		}
		for _, col := range table.Columns {
			for i := 1; i < col.Len(); i++ {
				col.Buckets[i].Count += col.Buckets[i-1].Count
			}
			col.PreCalculateScalar()
		}
	}
	return nil
}

// InitStats will init the stats cache using full load strategy.
func (h *Handle) InitStats(is infoschema.InfoSchema) error {
	tables, err := h.initStatsMeta(is)
	if err != nil {
		return errors.Trace(err)
	}
	err = h.initStatsHistograms(is, tables)
	if err != nil {
		return errors.Trace(err)
	}
	err = h.initStatsBuckets(tables)
	if err != nil {
		return errors.Trace(err)
	}
	h.statsCache.Store(tables)
	return nil
}

func getFullTableName(is infoschema.InfoSchema, tblInfo *model.TableInfo) string {
	for _, schema := range is.AllSchemas() {
		if t, err := is.TableByName(schema.Name, tblInfo.Name); err == nil {
			if t.Meta().ID == tblInfo.ID {
				return schema.Name.O + "." + tblInfo.Name.O
			}
		}
	}
	return fmt.Sprintf("%d", tblInfo.ID)
}
