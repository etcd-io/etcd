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

package tables

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/tablecodec"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/mock"
	log "github.com/sirupsen/logrus"
)

// Both partition and partitionedTable implement the table.Table interface.
var _ table.Table = &partition{}
var _ table.Table = &partitionedTable{}

// partitionedTable implements the table.PartitionedTable interface.
var _ table.PartitionedTable = &partitionedTable{}

// partition is a feature from MySQL:
// See https://dev.mysql.com/doc/refman/8.0/en/partitioning.html
// A partition table may contain many partitions, each partition has a unique partition
// id. The underlying representation of a partition and a normal table (a table with no
// partitions) is basically the same.
// partition also implements the table.Table interface.
type partition struct {
	tableCommon
}

// GetPhysicalID implements table.Table GetPhysicalID interface.
func (p *partition) GetPhysicalID() int64 {
	return p.physicalTableID
}

// partitionedTable implements the table.PartitionedTable interface.
// partitionedTable is a table, it contains many Partitions.
type partitionedTable struct {
	Table
	partitionExpr *PartitionExpr
	partitions    map[int64]*partition
}

func newPartitionedTable(tbl *Table, tblInfo *model.TableInfo) (table.Table, error) {
	partitionExpr, err := generatePartitionExpr(tblInfo)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err = initTableIndices(&tbl.tableCommon); err != nil {
		return nil, errors.Trace(err)
	}
	partitions := make(map[int64]*partition)
	pi := tblInfo.GetPartitionInfo()
	for _, p := range pi.Definitions {
		var t partition
		err = initTableCommonWithIndices(&t.tableCommon, tblInfo, p.ID, tbl.Columns, tbl.alloc)
		if err != nil {
			return nil, errors.Trace(err)
		}
		partitions[p.ID] = &t
	}

	return &partitionedTable{
		Table:         *tbl,
		partitionExpr: partitionExpr,
		partitions:    partitions,
	}, nil
}

// PartitionExpr is the partition definition expressions.
// There are two expressions exist, because Locate use binary search, which requires:
// Given a compare function, for any partition range i, if cmp[i] > 0, then cmp[i+1] > 0.
// While partition prune must use the accurate range to do prunning.
// partition by range (x)
//   (partition
//      p1 values less than (y1)
//      p2 values less than (y2)
//      p3 values less than (y3))
// Ranges: (x < y1 or x is null); (y1 <= x < y2); (y2 <= x < y3)
// UpperBounds: (x < y1); (x < y2); (x < y3)
type PartitionExpr struct {
	// Column is the column appeared in the by range expression, partition pruning need this to work.
	Column      *expression.Column
	Ranges      []expression.Expression
	UpperBounds []expression.Expression
}

func generatePartitionExpr(tblInfo *model.TableInfo) (*PartitionExpr, error) {
	var column *expression.Column
	// The caller should assure partition info is not nil.
	pi := tblInfo.GetPartitionInfo()
	ctx := mock.NewContext()
	partitionPruneExprs := make([]expression.Expression, 0, len(pi.Definitions))
	locateExprs := make([]expression.Expression, 0, len(pi.Definitions))
	var buf bytes.Buffer
	for i := 0; i < len(pi.Definitions); i++ {
		if strings.EqualFold(pi.Definitions[i].LessThan[0], "MAXVALUE") {
			// Expr less than maxvalue is always true.
			fmt.Fprintf(&buf, "true")
		} else {
			fmt.Fprintf(&buf, "((%s) < (%s))", pi.Expr, pi.Definitions[i].LessThan[0])
		}
		expr, err := expression.ParseSimpleExprWithTableInfo(ctx, buf.String(), tblInfo)
		if err != nil {
			// If it got an error here, ddl may hang forever, so this error log is important.
			log.Error("wrong table partition expression:", errors.ErrorStack(err), buf.String())
			return nil, errors.Trace(err)
		}
		locateExprs = append(locateExprs, expr)

		if i > 0 {
			fmt.Fprintf(&buf, " and ((%s) >= (%s))", pi.Expr, pi.Definitions[i-1].LessThan[0])
		} else {
			// NULL will locate in the first partition, so its expression is (expr < value or expr is null).
			fmt.Fprintf(&buf, " or ((%s) is null)", pi.Expr)

			// Extracts the column of the partition expression, it will be used by partition prunning.
			if tmp, err1 := expression.ParseSimpleExprWithTableInfo(ctx, pi.Expr, tblInfo); err1 == nil {
				if col, ok := tmp.(*expression.Column); ok {
					column = col
				}
			}
			if column == nil {
				log.Warnf("partition pruning won't work on this expr:%s", pi.Expr)
			}
		}

		expr, err = expression.ParseSimpleExprWithTableInfo(ctx, buf.String(), tblInfo)
		if err != nil {
			// If it got an error here, ddl may hang forever, so this error log is important.
			log.Error("wrong table partition expression:", errors.ErrorStack(err), buf.String())
			return nil, errors.Trace(err)
		}
		partitionPruneExprs = append(partitionPruneExprs, expr)
		buf.Reset()
	}
	return &PartitionExpr{
		Column:      column,
		Ranges:      partitionPruneExprs,
		UpperBounds: locateExprs,
	}, nil
}

// PartitionExpr returns the partition expression.
func (t *partitionedTable) PartitionExpr() *PartitionExpr {
	return t.partitionExpr
}

func partitionRecordKey(pid int64, handle int64) kv.Key {
	recordPrefix := tablecodec.GenTableRecordPrefix(pid)
	return tablecodec.EncodeRecordKey(recordPrefix, handle)
}

// locatePartition returns the partition ID of the input record.
func (t *partitionedTable) locatePartition(ctx sessionctx.Context, pi *model.PartitionInfo, r []types.Datum) (int64, error) {
	var err error
	var isNull bool
	partitionExprs := t.partitionExpr.UpperBounds
	idx := sort.Search(len(partitionExprs), func(i int) bool {
		var ret int64
		ret, isNull, err = partitionExprs[i].EvalInt(ctx, chunk.MutRowFromDatums(r).ToRow())
		if err != nil {
			return true // Break the search.
		}
		if isNull {
			// If the column value used to determine the partition is NULL, the row is inserted into the lowest partition.
			// See https://dev.mysql.com/doc/mysql-partitioning-excerpt/5.7/en/partitioning-handling-nulls.html
			return true // Break the search.
		}
		return ret > 0
	})
	if err != nil {
		return 0, errors.Trace(err)
	}
	if isNull {
		idx = 0
	}
	if idx < 0 || idx >= len(partitionExprs) {
		// The data does not belong to any of the partition?
		return 0, errors.Trace(table.ErrTrgInvalidCreationCtx)
	}
	return pi.Definitions[idx].ID, nil
}

// GetPartition returns a Table, which is actually a partition.
func (t *partitionedTable) GetPartition(pid int64) table.PhysicalTable {
	return t.partitions[pid]
}

// GetPartitionByRow returns a Table, which is actually a Partition.
func (t *partitionedTable) GetPartitionByRow(ctx sessionctx.Context, r []types.Datum) (table.Table, error) {
	pid, err := t.locatePartition(ctx, t.Meta().GetPartitionInfo(), r)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return t.partitions[pid], nil
}

// AddRecord implements the AddRecord method for the table.Table interface.
func (t *partitionedTable) AddRecord(ctx sessionctx.Context, r []types.Datum, skipHandleCheck bool) (recordID int64, err error) {
	partitionInfo := t.meta.GetPartitionInfo()
	pid, err := t.locatePartition(ctx, partitionInfo, r)
	if err != nil {
		return 0, errors.Trace(err)
	}

	tbl := t.GetPartition(pid)
	return tbl.AddRecord(ctx, r, skipHandleCheck)
}

// RemoveRecord implements table.Table RemoveRecord interface.
func (t *partitionedTable) RemoveRecord(ctx sessionctx.Context, h int64, r []types.Datum) error {
	partitionInfo := t.meta.GetPartitionInfo()
	pid, err := t.locatePartition(ctx, partitionInfo, r)
	if err != nil {
		return errors.Trace(err)
	}

	tbl := t.GetPartition(pid)
	return tbl.RemoveRecord(ctx, h, r)
}

// UpdateRecord implements table.Table UpdateRecord interface.
// `touched` means which columns are really modified, used for secondary indices.
// Length of `oldData` and `newData` equals to length of `t.WritableCols()`.
func (t *partitionedTable) UpdateRecord(ctx sessionctx.Context, h int64, currData, newData []types.Datum, touched []bool) error {
	partitionInfo := t.meta.GetPartitionInfo()
	from, err := t.locatePartition(ctx, partitionInfo, currData)
	if err != nil {
		return errors.Trace(err)
	}
	to, err := t.locatePartition(ctx, partitionInfo, newData)
	if err != nil {
		return errors.Trace(err)
	}

	// The old and new data locate in different partitions.
	// Remove record from old partition and add record to new partition.
	if from != to {
		_, err = t.GetPartition(to).AddRecord(ctx, newData, false)
		if err != nil {
			return errors.Trace(err)
		}
		// UpdateRecord should be side effect free, but there're two steps here.
		// What would happen if step1 succeed but step2 meets error? It's hard
		// to rollback.
		// So this special order is chosen: add record first, errors such as
		// 'Key Already Exists' will generally happen during step1, errors are
		// unlikely to happen in step2.
		err = t.GetPartition(from).RemoveRecord(ctx, h, currData)
		if err != nil {
			log.Error("partition update record error, it may write dirty data to txn:", errors.ErrorStack(err))
			return errors.Trace(err)
		}
		return nil
	}

	tbl := t.GetPartition(to)
	return tbl.UpdateRecord(ctx, h, currData, newData, touched)
}
