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

package statistics

import (
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tipb/go-tipb"
)

// JSONTable is used for dumping statistics.
type JSONTable struct {
	DatabaseName string                 `json:"database_name"`
	TableName    string                 `json:"table_name"`
	Columns      map[string]*jsonColumn `json:"columns"`
	Indices      map[string]*jsonColumn `json:"indices"`
	Count        int64                  `json:"count"`
	ModifyCount  int64                  `json:"modify_count"`
	Partitions   map[string]*JSONTable  `json:"partitions"`
}

type jsonColumn struct {
	Histogram         *tipb.Histogram `json:"histogram"`
	CMSketch          *tipb.CMSketch  `json:"cm_sketch"`
	NullCount         int64           `json:"null_count"`
	TotColSize        int64           `json:"tot_col_size"`
	LastUpdateVersion uint64          `json:"last_update_version"`
}

func dumpJSONCol(hist *Histogram, CMSketch *CMSketch) *jsonColumn {
	jsonCol := &jsonColumn{
		Histogram:         HistogramToProto(hist),
		NullCount:         hist.NullCount,
		TotColSize:        hist.TotColSize,
		LastUpdateVersion: hist.LastUpdateVersion,
	}
	if CMSketch != nil {
		jsonCol.CMSketch = CMSketchToProto(CMSketch)
	}
	return jsonCol
}

// DumpStatsToJSON dumps statistic to json.
func (h *Handle) DumpStatsToJSON(dbName string, tableInfo *model.TableInfo) (*JSONTable, error) {
	pi := tableInfo.GetPartitionInfo()
	if pi == nil {
		return h.tableStatsToJSON(dbName, tableInfo, tableInfo.ID)
	}
	jsonTbl := &JSONTable{
		DatabaseName: dbName,
		TableName:    tableInfo.Name.L,
		Partitions:   make(map[string]*JSONTable, len(pi.Definitions)),
	}
	for _, def := range pi.Definitions {
		tbl, err := h.tableStatsToJSON(dbName, tableInfo, def.ID)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if tbl == nil {
			continue
		}
		jsonTbl.Partitions[def.Name.L] = tbl
	}
	return jsonTbl, nil
}

func (h *Handle) tableStatsToJSON(dbName string, tableInfo *model.TableInfo, physicalID int64) (*JSONTable, error) {
	tbl, err := h.tableStatsFromStorage(tableInfo, physicalID, true)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if tbl == nil {
		return nil, nil
	}
	jsonTbl := &JSONTable{
		DatabaseName: dbName,
		TableName:    tableInfo.Name.L,
		Columns:      make(map[string]*jsonColumn, len(tbl.Columns)),
		Indices:      make(map[string]*jsonColumn, len(tbl.Indices)),
		Count:        tbl.Count,
		ModifyCount:  tbl.ModifyCount,
	}

	for _, col := range tbl.Columns {
		sc := &stmtctx.StatementContext{TimeZone: time.UTC}
		hist, err := col.ConvertTo(sc, types.NewFieldType(mysql.TypeBlob))
		if err != nil {
			return nil, errors.Trace(err)
		}
		jsonTbl.Columns[col.Info.Name.L] = dumpJSONCol(hist, col.CMSketch)
	}

	for _, idx := range tbl.Indices {
		jsonTbl.Indices[idx.Info.Name.L] = dumpJSONCol(&idx.Histogram, idx.CMSketch)
	}
	return jsonTbl, nil
}

// LoadStatsFromJSON will load statistic from JSONTable, and save it to the storage.
func (h *Handle) LoadStatsFromJSON(is infoschema.InfoSchema, jsonTbl *JSONTable) error {
	table, err := is.TableByName(model.NewCIStr(jsonTbl.DatabaseName), model.NewCIStr(jsonTbl.TableName))
	if err != nil {
		return errors.Trace(err)
	}
	tableInfo := table.Meta()
	pi := tableInfo.GetPartitionInfo()
	if pi == nil {
		err := h.loadStatsFromJSON(tableInfo, tableInfo.ID, jsonTbl)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		if jsonTbl.Partitions == nil {
			return errors.New("No partition statistics")
		}
		for _, def := range pi.Definitions {
			tbl := jsonTbl.Partitions[def.Name.L]
			if tbl == nil {
				continue
			}
			err := h.loadStatsFromJSON(tableInfo, def.ID, tbl)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return errors.Trace(h.Update(is))
}

func (h *Handle) loadStatsFromJSON(tableInfo *model.TableInfo, physicalID int64, jsonTbl *JSONTable) error {
	tbl, err := TableStatsFromJSON(tableInfo, physicalID, jsonTbl)
	if err != nil {
		return errors.Trace(err)
	}

	for _, col := range tbl.Columns {
		err = h.SaveStatsToStorage(tbl.PhysicalID, tbl.Count, 0, &col.Histogram, col.CMSketch, 1)
		if err != nil {
			return errors.Trace(err)
		}
	}
	for _, idx := range tbl.Indices {
		err = h.SaveStatsToStorage(tbl.PhysicalID, tbl.Count, 1, &idx.Histogram, idx.CMSketch, 1)
		if err != nil {
			return errors.Trace(err)
		}
	}
	err = h.SaveMetaToStorage(tbl.PhysicalID, tbl.Count, tbl.ModifyCount)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// TableStatsFromJSON loads statistic from JSONTable and return the Table of statistic.
func TableStatsFromJSON(tableInfo *model.TableInfo, physicalID int64, jsonTbl *JSONTable) (*Table, error) {
	newHistColl := HistColl{
		PhysicalID:     physicalID,
		HavePhysicalID: true,
		Count:          jsonTbl.Count,
		ModifyCount:    jsonTbl.ModifyCount,
		Columns:        make(map[int64]*Column, len(jsonTbl.Columns)),
		Indices:        make(map[int64]*Index, len(jsonTbl.Indices)),
	}
	tbl := &Table{
		HistColl: newHistColl,
	}
	for id, jsonIdx := range jsonTbl.Indices {
		for _, idxInfo := range tableInfo.Indices {
			if idxInfo.Name.L != id {
				continue
			}
			hist := HistogramFromProto(jsonIdx.Histogram)
			hist.ID, hist.NullCount, hist.LastUpdateVersion = idxInfo.ID, jsonIdx.NullCount, jsonIdx.LastUpdateVersion
			idx := &Index{
				Histogram: *hist,
				CMSketch:  CMSketchFromProto(jsonIdx.CMSketch),
				Info:      idxInfo,
			}
			tbl.Indices[idx.ID] = idx
		}
	}

	for id, jsonCol := range jsonTbl.Columns {
		for _, colInfo := range tableInfo.Columns {
			if colInfo.Name.L != id {
				continue
			}
			hist := HistogramFromProto(jsonCol.Histogram)
			count := int64(hist.totalRowCount())
			sc := &stmtctx.StatementContext{TimeZone: time.UTC}
			hist, err := hist.ConvertTo(sc, &colInfo.FieldType)
			if err != nil {
				return nil, errors.Trace(err)
			}
			hist.ID, hist.NullCount, hist.LastUpdateVersion, hist.TotColSize = colInfo.ID, jsonCol.NullCount, jsonCol.LastUpdateVersion, jsonCol.TotColSize
			col := &Column{
				Histogram: *hist,
				CMSketch:  CMSketchFromProto(jsonCol.CMSketch),
				Info:      colInfo,
				Count:     count,
				isHandle:  tableInfo.PKIsHandle && mysql.HasPriKeyFlag(colInfo.Flag),
			}
			tbl.Columns[col.ID] = col
		}
	}
	return tbl, nil
}
