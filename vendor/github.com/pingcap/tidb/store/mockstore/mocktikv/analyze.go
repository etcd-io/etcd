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

package mocktikv

import (
	"github.com/golang/protobuf/proto"
	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/coprocessor"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/statistics"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tipb/go-tipb"
	"golang.org/x/net/context"
)

func (h *rpcHandler) handleCopAnalyzeRequest(req *coprocessor.Request) *coprocessor.Response {
	resp := &coprocessor.Response{}
	if len(req.Ranges) == 0 {
		return resp
	}
	if req.GetTp() != kv.ReqTypeAnalyze {
		return resp
	}
	if err := h.checkRequestContext(req.GetContext()); err != nil {
		resp.RegionError = err
		return resp
	}
	analyzeReq := new(tipb.AnalyzeReq)
	err := proto.Unmarshal(req.Data, analyzeReq)
	if err != nil {
		resp.OtherError = err.Error()
		return resp
	}
	if analyzeReq.Tp == tipb.AnalyzeType_TypeIndex {
		resp, err = h.handleAnalyzeIndexReq(req, analyzeReq)
	} else {
		resp, err = h.handleAnalyzeColumnsReq(req, analyzeReq)
	}
	if err != nil {
		resp.OtherError = err.Error()
	}
	return resp
}

func (h *rpcHandler) handleAnalyzeIndexReq(req *coprocessor.Request, analyzeReq *tipb.AnalyzeReq) (*coprocessor.Response, error) {
	ranges, err := h.extractKVRanges(req.Ranges, false)
	if err != nil {
		return nil, errors.Trace(err)
	}
	e := &indexScanExec{
		colsLen:        int(analyzeReq.IdxReq.NumColumns),
		kvRanges:       ranges,
		startTS:        analyzeReq.StartTs,
		isolationLevel: h.isolationLevel,
		mvccStore:      h.mvccStore,
		IndexScan:      &tipb.IndexScan{Desc: false},
	}
	statsBuilder := statistics.NewSortedBuilder(flagsToStatementContext(analyzeReq.Flags), analyzeReq.IdxReq.BucketSize, 0, types.NewFieldType(mysql.TypeBlob))
	var cms *statistics.CMSketch
	if analyzeReq.IdxReq.CmsketchDepth != nil && analyzeReq.IdxReq.CmsketchWidth != nil {
		cms = statistics.NewCMSketch(*analyzeReq.IdxReq.CmsketchDepth, *analyzeReq.IdxReq.CmsketchWidth)
	}
	ctx := context.TODO()
	var values [][]byte
	for {
		values, err = e.Next(ctx)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if values == nil {
			break
		}
		var value []byte
		for _, val := range values {
			value = append(value, val...)
			if cms != nil {
				cms.InsertBytes(value)
			}
		}
		err = statsBuilder.Iterate(types.NewBytesDatum(value))
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	hg := statistics.HistogramToProto(statsBuilder.Hist())
	var cm *tipb.CMSketch
	if cms != nil {
		cm = statistics.CMSketchToProto(cms)
	}
	data, err := proto.Marshal(&tipb.AnalyzeIndexResp{Hist: hg, Cms: cm})
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &coprocessor.Response{Data: data}, nil
}

type analyzeColumnsExec struct {
	tblExec *tableScanExec
	fields  []*ast.ResultField
}

func (h *rpcHandler) handleAnalyzeColumnsReq(req *coprocessor.Request, analyzeReq *tipb.AnalyzeReq) (_ *coprocessor.Response, err error) {
	sc := flagsToStatementContext(analyzeReq.Flags)
	sc.TimeZone, err = constructTimeZone("", int(analyzeReq.TimeZoneOffset))
	if err != nil {
		return nil, errors.Trace(err)
	}

	evalCtx := &evalContext{sc: sc}
	columns := analyzeReq.ColReq.ColumnsInfo
	evalCtx.setColumnInfo(columns)
	ranges, err := h.extractKVRanges(req.Ranges, false)
	if err != nil {
		return nil, errors.Trace(err)
	}
	e := &analyzeColumnsExec{
		tblExec: &tableScanExec{
			TableScan:      &tipb.TableScan{Columns: columns},
			kvRanges:       ranges,
			colIDs:         evalCtx.colIDs,
			startTS:        analyzeReq.GetStartTs(),
			isolationLevel: h.isolationLevel,
			mvccStore:      h.mvccStore,
		},
	}
	e.fields = make([]*ast.ResultField, len(columns))
	for i := range e.fields {
		rf := new(ast.ResultField)
		rf.Column = new(model.ColumnInfo)
		rf.Column.FieldType = types.FieldType{Tp: mysql.TypeBlob, Flen: mysql.MaxBlobWidth, Charset: mysql.DefaultCharset, Collate: mysql.DefaultCollationName}
		e.fields[i] = rf
	}

	pkID := int64(-1)
	numCols := len(columns)
	if columns[0].GetPkHandle() {
		pkID = columns[0].ColumnId
		numCols--
	}
	colReq := analyzeReq.ColReq
	builder := statistics.SampleBuilder{
		Sc:              sc,
		RecordSet:       e,
		ColLen:          numCols,
		MaxBucketSize:   colReq.BucketSize,
		MaxFMSketchSize: colReq.SketchSize,
		MaxSampleSize:   colReq.SampleSize,
	}
	if pkID != -1 {
		builder.PkBuilder = statistics.NewSortedBuilder(sc, builder.MaxBucketSize, pkID, types.NewFieldType(mysql.TypeBlob))
	}
	if colReq.CmsketchWidth != nil && colReq.CmsketchDepth != nil {
		builder.CMSketchWidth = *colReq.CmsketchWidth
		builder.CMSketchDepth = *colReq.CmsketchDepth
	}
	collectors, pkBuilder, err := builder.CollectColumnStats()
	if err != nil {
		return nil, errors.Trace(err)
	}
	colResp := &tipb.AnalyzeColumnsResp{}
	if pkID != -1 {
		colResp.PkHist = statistics.HistogramToProto(pkBuilder.Hist())
	}
	for _, c := range collectors {
		colResp.Collectors = append(colResp.Collectors, statistics.SampleCollectorToProto(c))
	}
	data, err := proto.Marshal(colResp)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &coprocessor.Response{Data: data}, nil
}

// Fields implements the sqlexec.RecordSet Fields interface.
func (e *analyzeColumnsExec) Fields() []*ast.ResultField {
	return e.fields
}

func (e *analyzeColumnsExec) getNext(ctx context.Context) ([]types.Datum, error) {
	values, err := e.tblExec.Next(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if values == nil {
		return nil, nil
	}
	datumRow := make([]types.Datum, 0, len(values))
	for _, val := range values {
		d := types.NewBytesDatum(val)
		if len(val) == 1 && val[0] == codec.NilFlag {
			d.SetNull()
		}
		datumRow = append(datumRow, d)
	}
	return datumRow, nil
}

func (e *analyzeColumnsExec) Next(ctx context.Context, chk *chunk.Chunk) error {
	chk.Reset()
	row, err := e.getNext(ctx)
	if row == nil || err != nil {
		return errors.Trace(err)
	}
	for i := 0; i < len(row); i++ {
		chk.AppendDatum(i, &row[i])
	}
	return nil
}

func (e *analyzeColumnsExec) NewChunk() *chunk.Chunk {
	fields := make([]*types.FieldType, 0, len(e.fields))
	for _, field := range e.fields {
		fields = append(fields, &field.Column.FieldType)
	}
	return chunk.NewChunkWithCapacity(fields, 1)
}

// Close implements the sqlexec.RecordSet Close interface.
func (e *analyzeColumnsExec) Close() error {
	return nil
}
