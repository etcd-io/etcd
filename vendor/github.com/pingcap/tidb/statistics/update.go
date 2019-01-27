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
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/metrics"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/store/tikv/oracle"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/sqlexec"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type tableDeltaMap map[int64]variable.TableDelta

func (m tableDeltaMap) update(id int64, delta int64, count int64, colSize *map[int64]int64) {
	item := m[id]
	item.Delta += delta
	item.Count += count
	if item.ColSize == nil {
		item.ColSize = make(map[int64]int64)
	}
	if colSize != nil {
		for key, val := range *colSize {
			item.ColSize[key] += val
		}
	}
	m[id] = item
}

type errorRateDelta struct {
	PkID         int64
	PkErrorRate  *ErrorRate
	IdxErrorRate map[int64]*ErrorRate
}

type errorRateDeltaMap map[int64]errorRateDelta

func (m errorRateDeltaMap) update(tableID int64, histID int64, rate float64, isIndex bool) {
	item := m[tableID]
	if isIndex {
		if item.IdxErrorRate == nil {
			item.IdxErrorRate = make(map[int64]*ErrorRate)
		}
		if item.IdxErrorRate[histID] == nil {
			item.IdxErrorRate[histID] = &ErrorRate{}
		}
		item.IdxErrorRate[histID].update(rate)
	} else {
		if item.PkErrorRate == nil {
			item.PkID = histID
			item.PkErrorRate = &ErrorRate{}
		}
		item.PkErrorRate.update(rate)
	}
	m[tableID] = item
}

func (m errorRateDeltaMap) merge(deltaMap errorRateDeltaMap) {
	for tableID, item := range deltaMap {
		tbl := m[tableID]
		for histID, errorRate := range item.IdxErrorRate {
			if tbl.IdxErrorRate == nil {
				tbl.IdxErrorRate = make(map[int64]*ErrorRate)
			}
			if tbl.IdxErrorRate[histID] == nil {
				tbl.IdxErrorRate[histID] = &ErrorRate{}
			}
			tbl.IdxErrorRate[histID].merge(errorRate)
		}
		if item.PkErrorRate != nil {
			if tbl.PkErrorRate == nil {
				tbl.PkID = item.PkID
				tbl.PkErrorRate = &ErrorRate{}
			}
			tbl.PkErrorRate.merge(item.PkErrorRate)
		}
		m[tableID] = tbl
	}
}

func (m errorRateDeltaMap) clear(tableID int64, histID int64, isIndex bool) {
	item := m[tableID]
	if isIndex {
		delete(item.IdxErrorRate, histID)
	} else {
		item.PkErrorRate = nil
	}
	m[tableID] = item
}

func (h *Handle) merge(s *SessionStatsCollector) {
	s.Lock()
	defer s.Unlock()
	for id, item := range s.mapper {
		h.globalMap.update(id, item.Delta, item.Count, &item.ColSize)
	}
	h.mu.Lock()
	h.mu.rateMap.merge(s.rateMap)
	h.mu.Unlock()
	s.rateMap = make(errorRateDeltaMap)
	h.feedback = mergeQueryFeedback(h.feedback, s.feedback)
	s.mapper = make(tableDeltaMap)
	s.feedback = s.feedback[:0]
}

// SessionStatsCollector is a list item that holds the delta mapper. If you want to write or read mapper, you must lock it.
type SessionStatsCollector struct {
	sync.Mutex

	mapper   tableDeltaMap
	feedback []*QueryFeedback
	rateMap  errorRateDeltaMap
	prev     *SessionStatsCollector
	next     *SessionStatsCollector
	// deleted is set to true when a session is closed. Every time we sweep the list, we will remove the useless collector.
	deleted bool
}

// Delete only sets the deleted flag true, it will be deleted from list when DumpStatsDeltaToKV is called.
func (s *SessionStatsCollector) Delete() {
	s.Lock()
	defer s.Unlock()
	s.deleted = true
}

// Update will updates the delta and count for one table id.
func (s *SessionStatsCollector) Update(id int64, delta int64, count int64, colSize *map[int64]int64) {
	s.Lock()
	defer s.Unlock()
	s.mapper.update(id, delta, count, colSize)
}

func mergeQueryFeedback(lq []*QueryFeedback, rq []*QueryFeedback) []*QueryFeedback {
	for _, q := range rq {
		if len(lq) >= MaxQueryFeedbackCount {
			break
		}
		lq = append(lq, q)
	}
	return lq
}

var (
	// MinLogScanCount is the minimum scan count for a feedback to be logged.
	MinLogScanCount = int64(1000)
	// MinLogErrorRate is the minimum error rate for a feedback to be logged.
	MinLogErrorRate = 0.5
)

// StoreQueryFeedback will merges the feedback into stats collector.
func (s *SessionStatsCollector) StoreQueryFeedback(feedback interface{}, h *Handle) error {
	q := feedback.(*QueryFeedback)
	// TODO: If the error rate is small or actual scan count is small, we do not need to store the feed back.
	if !q.valid || q.hist == nil {
		return nil
	}
	err := q.recalculateExpectCount(h)
	if err != nil {
		return errors.Trace(err)
	}
	expected := float64(q.expected)
	var rate float64
	if q.actual == 0 {
		if expected == 0 {
			rate = 0
		} else {
			rate = 1
		}
	} else {
		rate = math.Abs(expected-float64(q.actual)) / float64(q.actual)
	}
	if rate >= MinLogErrorRate && (q.actual >= MinLogScanCount || q.expected >= MinLogScanCount) && log.GetLevel() == log.DebugLevel {
		q.logDetailedInfo(h)
	}
	metrics.StatsInaccuracyRate.Observe(rate)
	s.Lock()
	defer s.Unlock()
	isIndex := q.tp == indexType
	s.rateMap.update(q.tableID, q.hist.ID, rate, isIndex)
	if len(s.feedback) < MaxQueryFeedbackCount {
		s.feedback = append(s.feedback, q)
	}
	return nil
}

// tryToRemoveFromList will remove this collector from the list if it's deleted flag is set.
func (s *SessionStatsCollector) tryToRemoveFromList() {
	s.Lock()
	defer s.Unlock()
	if !s.deleted {
		return
	}
	next := s.next
	prev := s.prev
	prev.next = next
	if next != nil {
		next.prev = prev
	}
}

// NewSessionStatsCollector allocates a stats collector for a session.
func (h *Handle) NewSessionStatsCollector() *SessionStatsCollector {
	h.listHead.Lock()
	defer h.listHead.Unlock()
	newCollector := &SessionStatsCollector{
		mapper:  make(tableDeltaMap),
		rateMap: make(errorRateDeltaMap),
		next:    h.listHead.next,
		prev:    h.listHead,
	}
	if h.listHead.next != nil {
		h.listHead.next.prev = newCollector
	}
	h.listHead.next = newCollector
	return newCollector
}

var (
	// DumpStatsDeltaRatio is the lower bound of `Modify Count / Table Count` for stats delta to be dumped.
	DumpStatsDeltaRatio = 1 / 10000.0
	// dumpStatsMaxDuration is the max duration since last update.
	dumpStatsMaxDuration = time.Hour
)

// needDumpStatsDelta returns true when only updates a small portion of the table and the time since last update
// do not exceed one hour.
func needDumpStatsDelta(h *Handle, id int64, item variable.TableDelta, currentTime time.Time) bool {
	if item.InitTime.IsZero() {
		item.InitTime = currentTime
	}
	tbl, ok := h.statsCache.Load().(statsCache)[id]
	if !ok {
		// No need to dump if the stats is invalid.
		return false
	}
	if currentTime.Sub(item.InitTime) > dumpStatsMaxDuration {
		// Dump the stats to kv at least once an hour.
		return true
	}
	if tbl.Count == 0 || float64(item.Count)/float64(tbl.Count) > DumpStatsDeltaRatio {
		// Dump the stats when there are many modifications.
		return true
	}
	return false
}

const (
	// DumpAll indicates dump all the delta info in to kv
	DumpAll = true
	// DumpDelta indicates dump part of the delta info in to kv.
	DumpDelta = false
)

// DumpStatsDeltaToKV sweeps the whole list and updates the global map, then we dumps every table that held in map to KV.
// If the `dumpAll` is false, it will only dump that delta info that `Modify Count / Table Count` greater than a ratio.
func (h *Handle) DumpStatsDeltaToKV(dumpMode bool) error {
	h.listHead.Lock()
	for collector := h.listHead.next; collector != nil; collector = collector.next {
		collector.tryToRemoveFromList()
		h.merge(collector)
	}
	h.listHead.Unlock()
	currentTime := time.Now()
	for id, item := range h.globalMap {
		if dumpMode == DumpDelta && !needDumpStatsDelta(h, id, item, currentTime) {
			continue
		}
		updated, err := h.dumpTableStatCountToKV(id, item)
		if err != nil {
			return errors.Trace(err)
		}
		if updated {
			h.globalMap.update(id, -item.Delta, -item.Count, nil)
		}
		if err = h.dumpTableStatColSizeToKV(id, item); err != nil {
			return errors.Trace(err)
		}
		if updated {
			delete(h.globalMap, id)
		} else {
			m := h.globalMap[id]
			m.ColSize = nil
			h.globalMap[id] = m
		}
	}
	return nil
}

// dumpTableStatDeltaToKV dumps a single delta with some table to KV and updates the version.
func (h *Handle) dumpTableStatCountToKV(id int64, delta variable.TableDelta) (updated bool, err error) {
	if delta.Count == 0 {
		return true, nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	ctx := context.TODO()
	exec := h.mu.ctx.(sqlexec.SQLExecutor)
	_, err = exec.Execute(ctx, "begin")
	if err != nil {
		return false, errors.Trace(err)
	}
	defer func() {
		err = finishTransaction(context.Background(), exec, err)
	}()

	txn, err := h.mu.ctx.Txn(true)
	if err != nil {
		return false, errors.Trace(err)
	}
	startTS := txn.StartTS()
	var sql string
	if delta.Delta < 0 {
		sql = fmt.Sprintf("update mysql.stats_meta set version = %d, count = count - %d, modify_count = modify_count + %d where table_id = %d and count >= %d", startTS, -delta.Delta, delta.Count, id, -delta.Delta)
	} else {
		sql = fmt.Sprintf("update mysql.stats_meta set version = %d, count = count + %d, modify_count = modify_count + %d where table_id = %d", startTS, delta.Delta, delta.Count, id)
	}
	_, err = h.mu.ctx.(sqlexec.SQLExecutor).Execute(ctx, sql)
	if err != nil {
		return
	}
	updated = h.mu.ctx.GetSessionVars().StmtCtx.AffectedRows() > 0
	return
}

func (h *Handle) dumpTableStatColSizeToKV(id int64, delta variable.TableDelta) error {
	if len(delta.ColSize) == 0 {
		return nil
	}
	values := make([]string, 0, len(delta.ColSize))
	for histID, deltaColSize := range delta.ColSize {
		if deltaColSize == 0 {
			continue
		}
		values = append(values, fmt.Sprintf("(%d, 0, %d, 0, %d)", id, histID, deltaColSize))
	}
	if len(values) == 0 {
		return nil
	}
	sql := fmt.Sprintf("insert into mysql.stats_histograms (table_id, is_index, hist_id, distinct_count, tot_col_size) "+
		"values %s on duplicate key update tot_col_size = tot_col_size + values(tot_col_size)", strings.Join(values, ","))
	_, _, err := h.restrictedExec.ExecRestrictedSQL(nil, sql)
	return errors.Trace(err)
}

// DumpStatsFeedbackToKV dumps the stats feedback to KV.
func (h *Handle) DumpStatsFeedbackToKV() error {
	var err error
	var successCount int
	for _, fb := range h.feedback {
		if fb.tp == pkType {
			err = h.dumpFeedbackToKV(fb)
		} else {
			t, ok := h.statsCache.Load().(statsCache)[fb.tableID]
			if ok {
				err = dumpFeedbackForIndex(h, fb, t)
			}
		}
		if err != nil {
			break
		}
		successCount++
	}
	h.feedback = h.feedback[successCount:]
	return errors.Trace(err)
}

func (h *Handle) dumpFeedbackToKV(fb *QueryFeedback) error {
	vals, err := encodeFeedback(fb)
	if err != nil {
		log.Debugf("error occurred when encoding feedback, err: %s", errors.ErrorStack(err))
		return nil
	}
	var isIndex int64
	if fb.tp == indexType {
		isIndex = 1
	}
	sql := fmt.Sprintf("insert into mysql.stats_feedback (table_id, hist_id, is_index, feedback) values "+
		"(%d, %d, %d, X'%X')", fb.tableID, fb.hist.ID, isIndex, vals)
	h.mu.Lock()
	_, err = h.mu.ctx.(sqlexec.SQLExecutor).Execute(context.TODO(), sql)
	h.mu.Unlock()
	if err != nil {
		metrics.DumpFeedbackCounter.WithLabelValues(metrics.LblError).Inc()
	} else {
		metrics.DumpFeedbackCounter.WithLabelValues(metrics.LblOK).Inc()
	}
	return errors.Trace(err)
}

// UpdateStatsByLocalFeedback will update statistics by the local feedback.
// Currently, we dump the feedback with the period of 10 minutes, which means
// it takes 10 minutes for a feedback to take effect. However, we can use the
// feedback locally on this tidb-server, so it could be used more timely.
func (h *Handle) UpdateStatsByLocalFeedback(is infoschema.InfoSchema) {
	h.listHead.Lock()
	for collector := h.listHead.next; collector != nil; collector = collector.next {
		collector.tryToRemoveFromList()
		h.merge(collector)
	}
	h.listHead.Unlock()
	for _, fb := range h.feedback {
		table, ok := is.TableByID(fb.tableID)
		if !ok {
			continue
		}
		tblStats := h.GetTableStats(table.Meta())
		newTblStats := tblStats.copy()
		if fb.tp == indexType {
			idx, ok := tblStats.Indices[fb.hist.ID]
			if !ok || idx.Histogram.Len() == 0 {
				continue
			}
			newIdx := *idx
			eqFB, ranFB := splitFeedbackByQueryType(fb.feedback)
			newIdx.CMSketch = UpdateCMSketch(idx.CMSketch, eqFB)
			newIdx.Histogram = *UpdateHistogram(&idx.Histogram, &QueryFeedback{feedback: ranFB})
			newIdx.Histogram.PreCalculateScalar()
			newTblStats.Indices[fb.hist.ID] = &newIdx
		} else {
			col, ok := tblStats.Columns[fb.hist.ID]
			if !ok || col.Histogram.Len() == 0 {
				continue
			}
			newCol := *col
			// only use the range query to update primary key
			_, ranFB := splitFeedbackByQueryType(fb.feedback)
			newFB := &QueryFeedback{feedback: ranFB}
			newFB = newFB.decodeIntValues()
			newCol.Histogram = *UpdateHistogram(&col.Histogram, newFB)
			newTblStats.Columns[fb.hist.ID] = &newCol
		}
		h.UpdateTableStats([]*Table{newTblStats}, nil)
	}
}

// UpdateErrorRate updates the error rate of columns from h.rateMap to cache.
func (h *Handle) UpdateErrorRate(is infoschema.InfoSchema) {
	h.mu.Lock()
	tbls := make([]*Table, 0, len(h.mu.rateMap))
	for id, item := range h.mu.rateMap {
		table, ok := is.TableByID(id)
		if !ok {
			continue
		}
		tbl := h.GetTableStats(table.Meta()).copy()
		if item.PkErrorRate != nil && tbl.Columns[item.PkID] != nil {
			col := *tbl.Columns[item.PkID]
			col.ErrorRate.merge(item.PkErrorRate)
			tbl.Columns[item.PkID] = &col
		}
		for key, val := range item.IdxErrorRate {
			if tbl.Indices[key] == nil {
				continue
			}
			idx := *tbl.Indices[key]
			idx.ErrorRate.merge(val)
			tbl.Indices[key] = &idx
		}
		tbls = append(tbls, tbl)
		delete(h.mu.rateMap, id)
	}
	h.mu.Unlock()
	h.UpdateTableStats(tbls, nil)
}

// HandleUpdateStats update the stats using feedback.
func (h *Handle) HandleUpdateStats(is infoschema.InfoSchema) error {
	sql := "select table_id, hist_id, is_index, feedback from mysql.stats_feedback order by table_id, hist_id, is_index"
	rows, _, err := h.restrictedExec.ExecRestrictedSQL(nil, sql)
	if len(rows) == 0 || err != nil {
		return errors.Trace(err)
	}

	var groupedRows [][]chunk.Row
	preIdx := 0
	tableID, histID, isIndex := rows[0].GetInt64(0), rows[0].GetInt64(1), rows[0].GetInt64(2)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if row.GetInt64(0) != tableID || row.GetInt64(1) != histID || row.GetInt64(2) != isIndex {
			groupedRows = append(groupedRows, rows[preIdx:i])
			tableID, histID, isIndex = row.GetInt64(0), row.GetInt64(1), row.GetInt64(2)
			preIdx = i
		}
	}
	groupedRows = append(groupedRows, rows[preIdx:])

	for _, rows := range groupedRows {
		if err := h.handleSingleHistogramUpdate(is, rows); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// handleSingleHistogramUpdate updates the Histogram and CM Sketch using these feedbacks. All the feedbacks for
// the same index or column are gathered in `rows`.
func (h *Handle) handleSingleHistogramUpdate(is infoschema.InfoSchema, rows []chunk.Row) (err error) {
	physicalTableID, histID, isIndex := rows[0].GetInt64(0), rows[0].GetInt64(1), rows[0].GetInt64(2)
	defer func() {
		if err == nil {
			err = errors.Trace(h.deleteOutdatedFeedback(physicalTableID, histID, isIndex))
		}
	}()
	h.mu.Lock()
	table, ok := h.getTableByPhysicalID(is, physicalTableID)
	h.mu.Unlock()
	// The table has been deleted.
	if !ok {
		return nil
	}
	var tbl *Table
	if table.Meta().GetPartitionInfo() != nil {
		tbl = h.GetPartitionStats(table.Meta(), physicalTableID)
	} else {
		tbl = h.GetTableStats(table.Meta())
	}
	var cms *CMSketch
	var hist *Histogram
	if isIndex == 1 {
		idx, ok := tbl.Indices[histID]
		if ok && idx.Histogram.Len() > 0 {
			idxHist := idx.Histogram
			hist = &idxHist
			cms = idx.CMSketch.copy()
		}
	} else {
		col, ok := tbl.Columns[histID]
		if ok && col.Histogram.Len() > 0 {
			colHist := col.Histogram
			hist = &colHist
		}
	}
	// The column or index has been deleted.
	if hist == nil {
		return nil
	}
	q := &QueryFeedback{}
	for _, row := range rows {
		err1 := decodeFeedback(row.GetBytes(3), q, cms)
		if err1 != nil {
			log.Debugf("decode feedback failed, err: %v", errors.ErrorStack(err))
		}
	}
	err = h.dumpStatsUpdateToKV(physicalTableID, isIndex, q, hist, cms)
	return errors.Trace(err)
}

func (h *Handle) deleteOutdatedFeedback(tableID, histID, isIndex int64) error {
	h.mu.Lock()
	h.mu.ctx.GetSessionVars().BatchDelete = true
	sql := fmt.Sprintf("delete from mysql.stats_feedback where table_id = %d and hist_id = %d and is_index = %d", tableID, histID, isIndex)
	_, err := h.mu.ctx.(sqlexec.SQLExecutor).Execute(context.TODO(), sql)
	h.mu.ctx.GetSessionVars().BatchDelete = false
	h.mu.Unlock()
	return errors.Trace(err)
}

func (h *Handle) dumpStatsUpdateToKV(tableID, isIndex int64, q *QueryFeedback, hist *Histogram, cms *CMSketch) error {
	hist = UpdateHistogram(hist, q)
	err := h.SaveStatsToStorage(tableID, -1, int(isIndex), hist, cms, 0)
	metrics.UpdateStatsCounter.WithLabelValues(metrics.RetLabel(err)).Inc()
	return errors.Trace(err)
}

const (
	// StatsOwnerKey is the stats owner path that is saved to etcd.
	StatsOwnerKey = "/tidb/stats/owner"
	// StatsPrompt is the prompt for stats owner manager.
	StatsPrompt = "stats"
)

// AutoAnalyzeMinCnt means if the count of table is less than this value, we needn't do auto analyze.
var AutoAnalyzeMinCnt int64 = 1000

// TableAnalyzed checks if the table is analyzed.
func TableAnalyzed(tbl *Table) bool {
	for _, col := range tbl.Columns {
		if col.Count > 0 {
			return true
		}
	}
	for _, idx := range tbl.Indices {
		if idx.Histogram.Len() > 0 {
			return true
		}
	}
	return false
}

// withinTimePeriod tests whether `now` is between `start` and `end`.
func withinTimePeriod(start, end, now time.Time) bool {
	// Converts to UTC and only keeps the hour and minute info.
	start, end, now = start.UTC(), end.UTC(), now.UTC()
	start = time.Date(0, 0, 0, start.Hour(), start.Minute(), 0, 0, time.UTC)
	end = time.Date(0, 0, 0, end.Hour(), end.Minute(), 0, 0, time.UTC)
	now = time.Date(0, 0, 0, now.Hour(), now.Minute(), 0, 0, time.UTC)
	// for cases like from 00:00 to 06:00
	if end.Sub(start) >= 0 {
		return now.Sub(start) >= 0 && now.Sub(end) <= 0
	}
	// for cases like from 22:00 to 06:00
	return now.Sub(end) <= 0 || now.Sub(start) >= 0
}

// NeedAnalyzeTable checks if we need to analyze the table:
// 1. If the table has never been analyzed, we need to analyze it when it has
//    not been modified for a while.
// 2. If the table had been analyzed before, we need to analyze it when
//    "tbl.ModifyCount/tbl.Count > autoAnalyzeRatio".
// 3. The current time is between `start` and `end`.
func NeedAnalyzeTable(tbl *Table, limit time.Duration, autoAnalyzeRatio float64, start, end, now time.Time) bool {
	analyzed := TableAnalyzed(tbl)
	if !analyzed {
		t := time.Unix(0, oracle.ExtractPhysical(tbl.Version)*int64(time.Millisecond))
		return time.Since(t) >= limit
	}
	// Auto analyze is disabled.
	if autoAnalyzeRatio == 0 {
		return false
	}
	// No need to analyze it.
	if float64(tbl.ModifyCount)/float64(tbl.Count) <= autoAnalyzeRatio {
		return false
	}
	// Tests if current time is within the time period.
	return withinTimePeriod(start, end, now)
}

const (
	minAutoAnalyzeRatio = 0.3
)

func (h *Handle) getAutoAnalyzeParameters() map[string]string {
	sql := fmt.Sprintf("select variable_name, variable_value from mysql.global_variables where variable_name in ('%s', '%s', '%s')",
		variable.TiDBAutoAnalyzeRatio, variable.TiDBAutoAnalyzeStartTime, variable.TiDBAutoAnalyzeEndTime)
	rows, _, err := h.restrictedExec.ExecRestrictedSQL(nil, sql)
	if err != nil {
		return map[string]string{}
	}
	parameters := make(map[string]string)
	for _, row := range rows {
		parameters[row.GetString(0)] = row.GetString(1)
	}
	return parameters
}

func parseAutoAnalyzeRatio(ratio string) float64 {
	autoAnalyzeRatio, err := strconv.ParseFloat(ratio, 64)
	if err != nil {
		return variable.DefAutoAnalyzeRatio
	}
	if autoAnalyzeRatio > 0 {
		autoAnalyzeRatio = math.Max(autoAnalyzeRatio, minAutoAnalyzeRatio)
	}
	return autoAnalyzeRatio
}

func parseAnalyzePeriod(start, end string) (time.Time, time.Time, error) {
	if start == "" {
		start = variable.DefAutoAnalyzeStartTime
	}
	if end == "" {
		end = variable.DefAutoAnalyzeEndTime
	}
	s, err := time.ParseInLocation(variable.AnalyzeFullTimeFormat, start, time.UTC)
	if err != nil {
		return s, s, errors.Trace(err)
	}
	e, err := time.ParseInLocation(variable.AnalyzeFullTimeFormat, end, time.UTC)
	if err != nil {
		return s, e, errors.Trace(err)
	}
	return s, e, nil
}

// HandleAutoAnalyze analyzes the newly created table or index.
func (h *Handle) HandleAutoAnalyze(is infoschema.InfoSchema) error {
	dbs := is.AllSchemaNames()
	parameters := h.getAutoAnalyzeParameters()
	autoAnalyzeRatio := parseAutoAnalyzeRatio(parameters[variable.TiDBAutoAnalyzeRatio])
	start, end, err := parseAnalyzePeriod(parameters[variable.TiDBAutoAnalyzeStartTime], parameters[variable.TiDBAutoAnalyzeEndTime])
	if err != nil {
		return errors.Trace(err)
	}
	for _, db := range dbs {
		tbls := is.SchemaTables(model.NewCIStr(db))
		for _, tbl := range tbls {
			tblInfo := tbl.Meta()
			pi := tblInfo.GetPartitionInfo()
			tblName := "`" + db + "`.`" + tblInfo.Name.O + "`"
			if pi == nil {
				statsTbl := h.GetTableStats(tblInfo)
				sql := fmt.Sprintf("analyze table %s", tblName)
				analyzed, err := h.autoAnalyzeTable(tblInfo, statsTbl, start, end, autoAnalyzeRatio, sql)
				if analyzed {
					return err
				}
				continue
			}
			for _, def := range pi.Definitions {
				sql := fmt.Sprintf("analyze table %s partition `%s`", tblName, def.Name.O)
				statsTbl := h.GetPartitionStats(tblInfo, def.ID)
				analyzed, err := h.autoAnalyzeTable(tblInfo, statsTbl, start, end, autoAnalyzeRatio, sql)
				if analyzed {
					return err
				}
				continue
			}
		}
	}
	return nil
}

func (h *Handle) autoAnalyzeTable(tblInfo *model.TableInfo, statsTbl *Table, start, end time.Time, ratio float64, sql string) (bool, error) {
	if statsTbl.Pseudo || statsTbl.Count < AutoAnalyzeMinCnt {
		return false, nil
	}
	if NeedAnalyzeTable(statsTbl, 20*h.Lease, ratio, start, end, time.Now()) {
		log.Infof("[stats] auto %s now", sql)
		return true, h.execAutoAnalyze(sql)
	}
	for _, idx := range tblInfo.Indices {
		if idx.State != model.StatePublic {
			continue
		}
		if _, ok := statsTbl.Indices[idx.ID]; !ok {
			sql = fmt.Sprintf("%s index `%s`", sql, idx.Name.O)
			log.Infof("[stats] auto %s now", sql)
			return true, h.execAutoAnalyze(sql)
		}
	}
	return false, nil
}

func (h *Handle) execAutoAnalyze(sql string) error {
	startTime := time.Now()
	_, _, err := h.restrictedExec.ExecRestrictedSQL(nil, sql)
	metrics.AutoAnalyzeHistogram.Observe(time.Since(startTime).Seconds())
	if err != nil {
		metrics.AutoAnalyzeCounter.WithLabelValues("failed").Inc()
	} else {
		metrics.AutoAnalyzeCounter.WithLabelValues("succ").Inc()
	}
	return errors.Trace(err)
}
