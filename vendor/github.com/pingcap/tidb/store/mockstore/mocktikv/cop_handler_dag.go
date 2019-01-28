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
	"bytes"
	"io"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/coprocessor"
	"github.com/pingcap/kvproto/pkg/errorpb"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/kvproto/pkg/tikvpb"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/expression/aggregation"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/codec"
	mockpkg "github.com/pingcap/tidb/util/mock"
	"github.com/pingcap/tidb/util/timeutil"
	"github.com/pingcap/tipb/go-tipb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var dummySlice = make([]byte, 0)

type dagContext struct {
	dagReq    *tipb.DAGRequest
	keyRanges []*coprocessor.KeyRange
	evalCtx   *evalContext
}

func (h *rpcHandler) handleCopDAGRequest(req *coprocessor.Request) *coprocessor.Response {
	resp := &coprocessor.Response{}
	if err := h.checkRequestContext(req.GetContext()); err != nil {
		resp.RegionError = err
		return resp
	}
	dagCtx, e, dagReq, err := h.buildDAGExecutor(req)
	if err != nil {
		resp.OtherError = err.Error()
		return resp
	}

	var (
		chunks []tipb.Chunk
		rowCnt int
	)
	ctx := context.TODO()
	for {
		var row [][]byte
		row, err = e.Next(ctx)
		if err != nil {
			break
		}
		if row == nil {
			break
		}
		data := dummySlice
		for _, offset := range dagReq.OutputOffsets {
			data = append(data, row[offset]...)
		}
		chunks = appendRow(chunks, data, rowCnt)
		rowCnt++
	}
	warnings := dagCtx.evalCtx.sc.GetWarnings()
	return buildResp(chunks, e.Counts(), err, warnings)
}

func (h *rpcHandler) buildDAGExecutor(req *coprocessor.Request) (*dagContext, executor, *tipb.DAGRequest, error) {
	if len(req.Ranges) == 0 {
		return nil, nil, nil, errors.New("request range is null")
	}
	if req.GetTp() != kv.ReqTypeDAG {
		return nil, nil, nil, errors.Errorf("unsupported request type %d", req.GetTp())
	}

	dagReq := new(tipb.DAGRequest)
	err := proto.Unmarshal(req.Data, dagReq)
	if err != nil {
		return nil, nil, nil, errors.Trace(err)
	}

	sc := flagsToStatementContext(dagReq.Flags)
	sc.TimeZone, err = constructTimeZone(dagReq.TimeZoneName, int(dagReq.TimeZoneOffset))
	if err != nil {
		return nil, nil, nil, errors.Trace(err)
	}

	ctx := &dagContext{
		dagReq:    dagReq,
		keyRanges: req.Ranges,
		evalCtx:   &evalContext{sc: sc},
	}
	e, err := h.buildDAG(ctx, dagReq.Executors)
	if err != nil {
		return nil, nil, nil, errors.Trace(err)
	}
	return ctx, e, dagReq, err
}

// constructTimeZone constructs timezone by name first. When the timezone name
// is set, the daylight saving problem must be considered. Otherwise the
// timezone offset in seconds east of UTC is used to constructed the timezone.
func constructTimeZone(name string, offset int) (*time.Location, error) {
	if name != "" {
		return timeutil.LoadLocation(name)
	}

	return time.FixedZone("", offset), nil
}

func (h *rpcHandler) handleCopStream(ctx context.Context, req *coprocessor.Request) (tikvpb.Tikv_CoprocessorStreamClient, error) {
	dagCtx, e, dagReq, err := h.buildDAGExecutor(req)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &mockCopStreamClient{
		exec:   e,
		req:    dagReq,
		ctx:    ctx,
		dagCtx: dagCtx,
	}, nil
}

func (h *rpcHandler) buildExec(ctx *dagContext, curr *tipb.Executor) (executor, error) {
	var currExec executor
	var err error
	switch curr.GetTp() {
	case tipb.ExecType_TypeTableScan:
		currExec, err = h.buildTableScan(ctx, curr)
	case tipb.ExecType_TypeIndexScan:
		currExec, err = h.buildIndexScan(ctx, curr)
	case tipb.ExecType_TypeSelection:
		currExec, err = h.buildSelection(ctx, curr)
	case tipb.ExecType_TypeAggregation:
		currExec, err = h.buildHashAgg(ctx, curr)
	case tipb.ExecType_TypeStreamAgg:
		currExec, err = h.buildStreamAgg(ctx, curr)
	case tipb.ExecType_TypeTopN:
		currExec, err = h.buildTopN(ctx, curr)
	case tipb.ExecType_TypeLimit:
		currExec = &limitExec{limit: curr.Limit.GetLimit()}
	default:
		// TODO: Support other types.
		err = errors.Errorf("this exec type %v doesn't support yet.", curr.GetTp())
	}

	return currExec, errors.Trace(err)
}

func (h *rpcHandler) buildDAG(ctx *dagContext, executors []*tipb.Executor) (executor, error) {
	var src executor
	for i := 0; i < len(executors); i++ {
		curr, err := h.buildExec(ctx, executors[i])
		if err != nil {
			return nil, errors.Trace(err)
		}
		curr.SetSrcExec(src)
		src = curr
	}
	return src, nil
}

func (h *rpcHandler) buildTableScan(ctx *dagContext, executor *tipb.Executor) (*tableScanExec, error) {
	columns := executor.TblScan.Columns
	ctx.evalCtx.setColumnInfo(columns)
	ranges, err := h.extractKVRanges(ctx.keyRanges, executor.TblScan.Desc)
	if err != nil {
		return nil, errors.Trace(err)
	}

	e := &tableScanExec{
		TableScan:      executor.TblScan,
		kvRanges:       ranges,
		colIDs:         ctx.evalCtx.colIDs,
		startTS:        ctx.dagReq.GetStartTs(),
		isolationLevel: h.isolationLevel,
		mvccStore:      h.mvccStore,
	}
	if ctx.dagReq.CollectRangeCounts != nil && *ctx.dagReq.CollectRangeCounts {
		e.counts = make([]int64, len(ranges))
	}
	return e, nil
}

func (h *rpcHandler) buildIndexScan(ctx *dagContext, executor *tipb.Executor) (*indexScanExec, error) {
	var err error
	columns := executor.IdxScan.Columns
	ctx.evalCtx.setColumnInfo(columns)
	length := len(columns)
	pkStatus := pkColNotExists
	// The PKHandle column info has been collected in ctx.
	if columns[length-1].GetPkHandle() {
		if mysql.HasUnsignedFlag(uint(columns[length-1].GetFlag())) {
			pkStatus = pkColIsUnsigned
		} else {
			pkStatus = pkColIsSigned
		}
		columns = columns[:length-1]
	} else if columns[length-1].ColumnId == model.ExtraHandleID {
		pkStatus = pkColIsSigned
		columns = columns[:length-1]
	}
	ranges, err := h.extractKVRanges(ctx.keyRanges, executor.IdxScan.Desc)
	if err != nil {
		return nil, errors.Trace(err)
	}

	e := &indexScanExec{
		IndexScan:      executor.IdxScan,
		kvRanges:       ranges,
		colsLen:        len(columns),
		startTS:        ctx.dagReq.GetStartTs(),
		isolationLevel: h.isolationLevel,
		mvccStore:      h.mvccStore,
		pkStatus:       pkStatus,
	}
	if ctx.dagReq.CollectRangeCounts != nil && *ctx.dagReq.CollectRangeCounts {
		e.counts = make([]int64, len(ranges))
	}
	return e, nil
}

func (h *rpcHandler) buildSelection(ctx *dagContext, executor *tipb.Executor) (*selectionExec, error) {
	var err error
	var relatedColOffsets []int
	pbConds := executor.Selection.Conditions
	for _, cond := range pbConds {
		relatedColOffsets, err = extractOffsetsInExpr(cond, ctx.evalCtx.columnInfos, relatedColOffsets)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	conds, err := convertToExprs(ctx.evalCtx.sc, ctx.evalCtx.fieldTps, pbConds)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &selectionExec{
		evalCtx:           ctx.evalCtx,
		relatedColOffsets: relatedColOffsets,
		conditions:        conds,
		row:               make([]types.Datum, len(ctx.evalCtx.columnInfos)),
	}, nil
}

func (h *rpcHandler) getAggInfo(ctx *dagContext, executor *tipb.Executor) ([]aggregation.Aggregation, []expression.Expression, []int, error) {
	length := len(executor.Aggregation.AggFunc)
	aggs := make([]aggregation.Aggregation, 0, length)
	var err error
	var relatedColOffsets []int
	for _, expr := range executor.Aggregation.AggFunc {
		var aggExpr aggregation.Aggregation
		aggExpr, err = aggregation.NewDistAggFunc(expr, ctx.evalCtx.fieldTps, ctx.evalCtx.sc)
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
		}
		aggs = append(aggs, aggExpr)
		relatedColOffsets, err = extractOffsetsInExpr(expr, ctx.evalCtx.columnInfos, relatedColOffsets)
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
		}
	}
	for _, item := range executor.Aggregation.GroupBy {
		relatedColOffsets, err = extractOffsetsInExpr(item, ctx.evalCtx.columnInfos, relatedColOffsets)
		if err != nil {
			return nil, nil, nil, errors.Trace(err)
		}
	}
	groupBys, err := convertToExprs(ctx.evalCtx.sc, ctx.evalCtx.fieldTps, executor.Aggregation.GetGroupBy())
	if err != nil {
		return nil, nil, nil, errors.Trace(err)
	}

	return aggs, groupBys, relatedColOffsets, nil
}

func (h *rpcHandler) buildHashAgg(ctx *dagContext, executor *tipb.Executor) (*hashAggExec, error) {
	aggs, groupBys, relatedColOffsets, err := h.getAggInfo(ctx, executor)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &hashAggExec{
		evalCtx:           ctx.evalCtx,
		aggExprs:          aggs,
		groupByExprs:      groupBys,
		groups:            make(map[string]struct{}),
		groupKeys:         make([][]byte, 0),
		relatedColOffsets: relatedColOffsets,
		row:               make([]types.Datum, len(ctx.evalCtx.columnInfos)),
	}, nil
}

func (h *rpcHandler) buildStreamAgg(ctx *dagContext, executor *tipb.Executor) (*streamAggExec, error) {
	aggs, groupBys, relatedColOffsets, err := h.getAggInfo(ctx, executor)
	if err != nil {
		return nil, errors.Trace(err)
	}
	aggCtxs := make([]*aggregation.AggEvaluateContext, 0, len(aggs))
	for _, agg := range aggs {
		aggCtxs = append(aggCtxs, agg.CreateContext(ctx.evalCtx.sc))
	}

	return &streamAggExec{
		evalCtx:           ctx.evalCtx,
		aggExprs:          aggs,
		aggCtxs:           aggCtxs,
		groupByExprs:      groupBys,
		currGroupByValues: make([][]byte, 0),
		relatedColOffsets: relatedColOffsets,
		row:               make([]types.Datum, len(ctx.evalCtx.columnInfos)),
	}, nil
}

func (h *rpcHandler) buildTopN(ctx *dagContext, executor *tipb.Executor) (*topNExec, error) {
	topN := executor.TopN
	var err error
	var relatedColOffsets []int
	pbConds := make([]*tipb.Expr, len(topN.OrderBy))
	for i, item := range topN.OrderBy {
		relatedColOffsets, err = extractOffsetsInExpr(item.Expr, ctx.evalCtx.columnInfos, relatedColOffsets)
		if err != nil {
			return nil, errors.Trace(err)
		}
		pbConds[i] = item.Expr
	}
	heap := &topNHeap{
		totalCount: int(topN.Limit),
		topNSorter: topNSorter{
			orderByItems: topN.OrderBy,
			sc:           ctx.evalCtx.sc,
		},
	}

	conds, err := convertToExprs(ctx.evalCtx.sc, ctx.evalCtx.fieldTps, pbConds)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &topNExec{
		heap:              heap,
		evalCtx:           ctx.evalCtx,
		relatedColOffsets: relatedColOffsets,
		orderByExprs:      conds,
		row:               make([]types.Datum, len(ctx.evalCtx.columnInfos)),
	}, nil
}

type evalContext struct {
	colIDs      map[int64]int
	columnInfos []*tipb.ColumnInfo
	fieldTps    []*types.FieldType
	sc          *stmtctx.StatementContext
}

func (e *evalContext) setColumnInfo(cols []*tipb.ColumnInfo) {
	e.columnInfos = make([]*tipb.ColumnInfo, len(cols))
	copy(e.columnInfos, cols)

	e.colIDs = make(map[int64]int)
	e.fieldTps = make([]*types.FieldType, 0, len(e.columnInfos))
	for i, col := range e.columnInfos {
		ft := fieldTypeFromPBColumn(col)
		e.fieldTps = append(e.fieldTps, ft)
		e.colIDs[col.GetColumnId()] = i
	}
}

// decodeRelatedColumnVals decodes data to Datum slice according to the row information.
func (e *evalContext) decodeRelatedColumnVals(relatedColOffsets []int, value [][]byte, row []types.Datum) error {
	var err error
	for _, offset := range relatedColOffsets {
		row[offset], err = tablecodec.DecodeColumnValue(value[offset], e.fieldTps[offset], e.sc.TimeZone)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// flagsToStatementContext creates a StatementContext from a `tipb.SelectRequest.Flags`.
func flagsToStatementContext(flags uint64) *stmtctx.StatementContext {
	sc := new(stmtctx.StatementContext)
	sc.IgnoreTruncate = (flags & model.FlagIgnoreTruncate) > 0
	sc.TruncateAsWarning = (flags & model.FlagTruncateAsWarning) > 0
	sc.PadCharToFullLength = (flags & model.FlagPadCharToFullLength) > 0
	return sc
}

// MockGRPCClientStream is exported for testing purpose.
func MockGRPCClientStream() grpc.ClientStream {
	return mockClientStream{}
}

// mockClientStream implements grpc ClientStream interface, its methods are never called.
type mockClientStream struct{}

func (mockClientStream) Header() (metadata.MD, error) { return nil, nil }
func (mockClientStream) Trailer() metadata.MD         { return nil }
func (mockClientStream) CloseSend() error             { return nil }
func (mockClientStream) Context() context.Context     { return nil }
func (mockClientStream) SendMsg(m interface{}) error  { return nil }
func (mockClientStream) RecvMsg(m interface{}) error  { return nil }

type mockCopStreamClient struct {
	mockClientStream

	req      *tipb.DAGRequest
	exec     executor
	ctx      context.Context
	dagCtx   *dagContext
	finished bool
}

type mockCopStreamErrClient struct {
	mockClientStream

	*errorpb.Error
}

func (mock *mockCopStreamErrClient) Recv() (*coprocessor.Response, error) {
	return &coprocessor.Response{
		RegionError: mock.Error,
	}, nil
}

func (mock *mockCopStreamClient) Recv() (*coprocessor.Response, error) {
	select {
	case <-mock.ctx.Done():
		return nil, mock.ctx.Err()
	default:
	}

	if mock.finished {
		return nil, io.EOF
	}

	if hook := mock.ctx.Value(mockpkg.HookKeyForTest("mockTiKVStreamRecvHook")); hook != nil {
		hook.(func(context.Context))(mock.ctx)
	}

	var resp coprocessor.Response
	counts := make([]int64, len(mock.req.Executors))
	chunk, finish, ran, counts, warnings, err := mock.readBlockFromExecutor()
	resp.Range = ran
	if err != nil {
		if locked, ok := errors.Cause(err).(*ErrLocked); ok {
			resp.Locked = &kvrpcpb.LockInfo{
				Key:         locked.Key,
				PrimaryLock: locked.Primary,
				LockVersion: locked.StartTS,
				LockTtl:     locked.TTL,
			}
		} else {
			resp.OtherError = err.Error()
		}
		return &resp, nil
	}
	if finish {
		// Just mark it, need to handle the last chunk.
		mock.finished = true
	}

	data, err := chunk.Marshal()
	if err != nil {
		resp.OtherError = err.Error()
		return &resp, nil
	}
	var Warnings []*tipb.Error
	if len(warnings) > 0 {
		Warnings = make([]*tipb.Error, 0, len(warnings))
		for i := range warnings {
			Warnings = append(Warnings, toPBError(warnings[i].Err))
		}
	}
	streamResponse := tipb.StreamResponse{
		Error:    toPBError(err),
		Data:     data,
		Warnings: Warnings,
	}
	// The counts was the output count of each executor, but now it is the scan count of each range,
	// so we need a flag to tell them apart.
	if counts != nil {
		streamResponse.OutputCounts = make([]int64, 1+len(counts))
		copy(streamResponse.OutputCounts, counts)
		streamResponse.OutputCounts[len(counts)] = -1
	}
	resp.Data, err = proto.Marshal(&streamResponse)
	if err != nil {
		resp.OtherError = err.Error()
	}
	return &resp, nil
}

func (mock *mockCopStreamClient) readBlockFromExecutor() (tipb.Chunk, bool, *coprocessor.KeyRange, []int64, []stmtctx.SQLWarn, error) {
	var chunk tipb.Chunk
	var ran coprocessor.KeyRange
	var finish bool
	var desc bool
	mock.exec.ResetCounts()
	ran.Start, desc = mock.exec.Cursor()
	for count := 0; count < rowsPerChunk; count++ {
		row, err := mock.exec.Next(mock.ctx)
		if err != nil {
			ran.End, _ = mock.exec.Cursor()
			return chunk, false, &ran, nil, nil, errors.Trace(err)
		}
		if row == nil {
			finish = true
			break
		}
		for _, offset := range mock.req.OutputOffsets {
			chunk.RowsData = append(chunk.RowsData, row[offset]...)
		}
	}

	ran.End, _ = mock.exec.Cursor()
	if desc {
		ran.Start, ran.End = ran.End, ran.Start
	}
	warnings := mock.dagCtx.evalCtx.sc.GetWarnings()
	mock.dagCtx.evalCtx.sc.SetWarnings(nil)
	return chunk, finish, &ran, mock.exec.Counts(), warnings, nil
}

func buildResp(chunks []tipb.Chunk, counts []int64, err error, warnings []stmtctx.SQLWarn) *coprocessor.Response {
	resp := &coprocessor.Response{}
	selResp := &tipb.SelectResponse{
		Error:        toPBError(err),
		Chunks:       chunks,
		OutputCounts: counts,
	}
	if len(warnings) > 0 {
		selResp.Warnings = make([]*tipb.Error, 0, len(warnings))
		for i := range warnings {
			selResp.Warnings = append(selResp.Warnings, toPBError(warnings[i].Err))
		}
	}
	if err != nil {
		if locked, ok := errors.Cause(err).(*ErrLocked); ok {
			resp.Locked = &kvrpcpb.LockInfo{
				Key:         locked.Key,
				PrimaryLock: locked.Primary,
				LockVersion: locked.StartTS,
				LockTtl:     locked.TTL,
			}
		} else {
			resp.OtherError = err.Error()
		}
	}
	data, err := proto.Marshal(selResp)
	if err != nil {
		resp.OtherError = err.Error()
		return resp
	}
	resp.Data = data
	return resp
}

func toPBError(err error) *tipb.Error {
	if err == nil {
		return nil
	}
	perr := new(tipb.Error)
	switch x := err.(type) {
	case *terror.Error:
		sqlErr := x.ToSQLError()
		perr.Code = int32(sqlErr.Code)
		perr.Msg = sqlErr.Message
	default:
		perr.Code = int32(1)
		perr.Msg = err.Error()
	}
	return perr
}

// extractKVRanges extracts kv.KeyRanges slice from a SelectRequest.
func (h *rpcHandler) extractKVRanges(keyRanges []*coprocessor.KeyRange, descScan bool) (kvRanges []kv.KeyRange, err error) {
	for _, kran := range keyRanges {
		if bytes.Compare(kran.GetStart(), kran.GetEnd()) >= 0 {
			err = errors.Errorf("invalid range, start should be smaller than end: %v %v", kran.GetStart(), kran.GetEnd())
			return
		}

		upperKey := kran.GetEnd()
		if bytes.Compare(upperKey, h.rawStartKey) <= 0 {
			continue
		}
		lowerKey := kran.GetStart()
		if len(h.rawEndKey) != 0 && bytes.Compare(lowerKey, h.rawEndKey) >= 0 {
			break
		}
		var kvr kv.KeyRange
		kvr.StartKey = kv.Key(maxStartKey(lowerKey, h.rawStartKey))
		kvr.EndKey = kv.Key(minEndKey(upperKey, h.rawEndKey))
		kvRanges = append(kvRanges, kvr)
	}
	if descScan {
		reverseKVRanges(kvRanges)
	}
	return
}

func reverseKVRanges(kvRanges []kv.KeyRange) {
	for i := 0; i < len(kvRanges)/2; i++ {
		j := len(kvRanges) - i - 1
		kvRanges[i], kvRanges[j] = kvRanges[j], kvRanges[i]
	}
}

const rowsPerChunk = 64

func appendRow(chunks []tipb.Chunk, data []byte, rowCnt int) []tipb.Chunk {
	if rowCnt%rowsPerChunk == 0 {
		chunks = append(chunks, tipb.Chunk{})
	}
	cur := &chunks[len(chunks)-1]
	cur.RowsData = append(cur.RowsData, data...)
	return chunks
}

func maxStartKey(rangeStartKey kv.Key, regionStartKey []byte) []byte {
	if bytes.Compare([]byte(rangeStartKey), regionStartKey) > 0 {
		return []byte(rangeStartKey)
	}
	return regionStartKey
}

func minEndKey(rangeEndKey kv.Key, regionEndKey []byte) []byte {
	if len(regionEndKey) == 0 || bytes.Compare([]byte(rangeEndKey), regionEndKey) < 0 {
		return []byte(rangeEndKey)
	}
	return regionEndKey
}

func isDuplicated(offsets []int, offset int) bool {
	for _, idx := range offsets {
		if idx == offset {
			return true
		}
	}
	return false
}

func extractOffsetsInExpr(expr *tipb.Expr, columns []*tipb.ColumnInfo, collector []int) ([]int, error) {
	if expr == nil {
		return nil, nil
	}
	if expr.GetTp() == tipb.ExprType_ColumnRef {
		_, idx, err := codec.DecodeInt(expr.Val)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !isDuplicated(collector, int(idx)) {
			collector = append(collector, int(idx))
		}
		return collector, nil
	}
	var err error
	for _, child := range expr.Children {
		collector, err = extractOffsetsInExpr(child, columns, collector)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return collector, nil
}

// fieldTypeFromPBColumn creates a types.FieldType from tipb.ColumnInfo.
func fieldTypeFromPBColumn(col *tipb.ColumnInfo) *types.FieldType {
	return &types.FieldType{
		Tp:      byte(col.GetTp()),
		Flag:    uint(col.Flag),
		Flen:    int(col.GetColumnLen()),
		Decimal: int(col.GetDecimal()),
		Elems:   col.Elems,
		Collate: mysql.Collations[uint8(col.GetCollation())],
	}
}
