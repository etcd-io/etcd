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

package infoschema

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/charset"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta/autoid"
	"github.com/pingcap/tidb/privilege"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/table"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/sqlexec"
)

const (
	tableSchemata                           = "SCHEMATA"
	tableTables                             = "TABLES"
	tableColumns                            = "COLUMNS"
	tableStatistics                         = "STATISTICS"
	tableCharacterSets                      = "CHARACTER_SETS"
	tableCollations                         = "COLLATIONS"
	tableFiles                              = "FILES"
	catalogVal                              = "def"
	tableProfiling                          = "PROFILING"
	tablePartitions                         = "PARTITIONS"
	tableKeyColumm                          = "KEY_COLUMN_USAGE"
	tableReferConst                         = "REFERENTIAL_CONSTRAINTS"
	tableSessionVar                         = "SESSION_VARIABLES"
	tablePlugins                            = "PLUGINS"
	tableConstraints                        = "TABLE_CONSTRAINTS"
	tableTriggers                           = "TRIGGERS"
	tableUserPrivileges                     = "USER_PRIVILEGES"
	tableSchemaPrivileges                   = "SCHEMA_PRIVILEGES"
	tableTablePrivileges                    = "TABLE_PRIVILEGES"
	tableColumnPrivileges                   = "COLUMN_PRIVILEGES"
	tableEngines                            = "ENGINES"
	tableViews                              = "VIEWS"
	tableRoutines                           = "ROUTINES"
	tableParameters                         = "PARAMETERS"
	tableEvents                             = "EVENTS"
	tableGlobalStatus                       = "GLOBAL_STATUS"
	tableGlobalVariables                    = "GLOBAL_VARIABLES"
	tableSessionStatus                      = "SESSION_STATUS"
	tableOptimizerTrace                     = "OPTIMIZER_TRACE"
	tableTableSpaces                        = "TABLESPACES"
	tableCollationCharacterSetApplicability = "COLLATION_CHARACTER_SET_APPLICABILITY"
	tableProcesslist                        = "PROCESSLIST"
)

type columnInfo struct {
	name  string
	tp    byte
	size  int
	flag  uint
	deflt interface{}
	elems []string
}

func buildColumnInfo(tableName string, col columnInfo) *model.ColumnInfo {
	mCharset := charset.CharsetBin
	mCollation := charset.CharsetBin
	mFlag := mysql.UnsignedFlag
	if col.tp == mysql.TypeVarchar || col.tp == mysql.TypeBlob {
		mCharset = charset.CharsetUTF8MB4
		mCollation = charset.CollationUTF8MB4
		mFlag = col.flag
	}
	fieldType := types.FieldType{
		Charset: mCharset,
		Collate: mCollation,
		Tp:      col.tp,
		Flen:    col.size,
		Flag:    mFlag,
	}
	return &model.ColumnInfo{
		Name:      model.NewCIStr(col.name),
		FieldType: fieldType,
		State:     model.StatePublic,
	}
}

func buildTableMeta(tableName string, cs []columnInfo) *model.TableInfo {
	cols := make([]*model.ColumnInfo, 0, len(cs))
	for _, c := range cs {
		cols = append(cols, buildColumnInfo(tableName, c))
	}
	for i, col := range cols {
		col.Offset = i
	}
	return &model.TableInfo{
		Name:    model.NewCIStr(tableName),
		Columns: cols,
		State:   model.StatePublic,
		Charset: mysql.DefaultCharset,
		Collate: mysql.DefaultCollationName,
	}
}

var schemataCols = []columnInfo{
	{"CATALOG_NAME", mysql.TypeVarchar, 512, 0, nil, nil},
	{"SCHEMA_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DEFAULT_CHARACTER_SET_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DEFAULT_COLLATION_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"SQL_PATH", mysql.TypeVarchar, 512, 0, nil, nil},
}

var tablesCols = []columnInfo{
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_TYPE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"ENGINE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"VERSION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"ROW_FORMAT", mysql.TypeVarchar, 10, 0, nil, nil},
	{"TABLE_ROWS", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AVG_ROW_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATA_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"MAX_DATA_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"INDEX_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATA_FREE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AUTO_INCREMENT", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATE_TIME", mysql.TypeDatetime, 19, 0, nil, nil},
	{"UPDATE_TIME", mysql.TypeDatetime, 19, 0, nil, nil},
	{"CHECK_TIME", mysql.TypeDatetime, 19, 0, nil, nil},
	{"TABLE_COLLATION", mysql.TypeVarchar, 32, mysql.NotNullFlag, "utf8_bin", nil},
	{"CHECK_SUM", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATE_OPTIONS", mysql.TypeVarchar, 255, 0, nil, nil},
	{"TABLE_COMMENT", mysql.TypeVarchar, 2048, 0, nil, nil},
}

// See: http://dev.mysql.com/doc/refman/5.7/en/columns-table.html
var columnsCols = []columnInfo{
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"COLUMN_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"ORDINAL_POSITION", mysql.TypeLonglong, 64, 0, nil, nil},
	{"COLUMN_DEFAULT", mysql.TypeBlob, 196606, 0, nil, nil},
	{"IS_NULLABLE", mysql.TypeVarchar, 3, 0, nil, nil},
	{"DATA_TYPE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"CHARACTER_MAXIMUM_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CHARACTER_OCTET_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"NUMERIC_PRECISION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"NUMERIC_SCALE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATETIME_PRECISION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"COLLATION_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"COLUMN_TYPE", mysql.TypeBlob, 196606, 0, nil, nil},
	{"COLUMN_KEY", mysql.TypeVarchar, 3, 0, nil, nil},
	{"EXTRA", mysql.TypeVarchar, 30, 0, nil, nil},
	{"PRIVILEGES", mysql.TypeVarchar, 80, 0, nil, nil},
	{"COLUMN_COMMENT", mysql.TypeVarchar, 1024, 0, nil, nil},
	{"GENERATION_EXPRESSION", mysql.TypeBlob, 589779, mysql.NotNullFlag, nil, nil},
}

var statisticsCols = []columnInfo{
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"NON_UNIQUE", mysql.TypeVarchar, 1, 0, nil, nil},
	{"INDEX_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"INDEX_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"SEQ_IN_INDEX", mysql.TypeLonglong, 2, 0, nil, nil},
	{"COLUMN_NAME", mysql.TypeVarchar, 21, 0, nil, nil},
	{"COLLATION", mysql.TypeVarchar, 1, 0, nil, nil},
	{"CARDINALITY", mysql.TypeLonglong, 21, 0, nil, nil},
	{"SUB_PART", mysql.TypeLonglong, 3, 0, nil, nil},
	{"PACKED", mysql.TypeVarchar, 10, 0, nil, nil},
	{"NULLABLE", mysql.TypeVarchar, 3, 0, nil, nil},
	{"INDEX_TYPE", mysql.TypeVarchar, 16, 0, nil, nil},
	{"COMMENT", mysql.TypeVarchar, 16, 0, nil, nil},
	{"INDEX_COMMENT", mysql.TypeVarchar, 1024, 0, nil, nil},
}

var profilingCols = []columnInfo{
	{"QUERY_ID", mysql.TypeLong, 20, 0, nil, nil},
	{"SEQ", mysql.TypeLong, 20, 0, nil, nil},
	{"STATE", mysql.TypeVarchar, 30, 0, nil, nil},
	{"DURATION", mysql.TypeNewDecimal, 9, 0, nil, nil},
	{"CPU_USER", mysql.TypeNewDecimal, 9, 0, nil, nil},
	{"CPU_SYSTEM", mysql.TypeNewDecimal, 9, 0, nil, nil},
	{"CONTEXT_VOLUNTARY", mysql.TypeLong, 20, 0, nil, nil},
	{"CONTEXT_INVOLUNTARY", mysql.TypeLong, 20, 0, nil, nil},
	{"BLOCK_OPS_IN", mysql.TypeLong, 20, 0, nil, nil},
	{"BLOCK_OPS_OUT", mysql.TypeLong, 20, 0, nil, nil},
	{"MESSAGES_SENT", mysql.TypeLong, 20, 0, nil, nil},
	{"MESSAGES_RECEIVED", mysql.TypeLong, 20, 0, nil, nil},
	{"PAGE_FAULTS_MAJOR", mysql.TypeLong, 20, 0, nil, nil},
	{"PAGE_FAULTS_MINOR", mysql.TypeLong, 20, 0, nil, nil},
	{"SWAPS", mysql.TypeLong, 20, 0, nil, nil},
	{"SOURCE_FUNCTION", mysql.TypeVarchar, 30, 0, nil, nil},
	{"SOURCE_FILE", mysql.TypeVarchar, 20, 0, nil, nil},
	{"SOURCE_LINE", mysql.TypeLong, 20, 0, nil, nil},
}

var charsetCols = []columnInfo{
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"DEFAULT_COLLATE_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"DESCRIPTION", mysql.TypeVarchar, 60, 0, nil, nil},
	{"MAXLEN", mysql.TypeLonglong, 3, 0, nil, nil},
}

var collationsCols = []columnInfo{
	{"COLLATION_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 32, 0, nil, nil},
	{"ID", mysql.TypeLonglong, 11, 0, nil, nil},
	{"IS_DEFAULT", mysql.TypeVarchar, 3, 0, nil, nil},
	{"IS_COMPILED", mysql.TypeVarchar, 3, 0, nil, nil},
	{"SORTLEN", mysql.TypeLonglong, 3, 0, nil, nil},
}

var keyColumnUsageCols = []columnInfo{
	{"CONSTRAINT_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"CONSTRAINT_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"CONSTRAINT_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"COLUMN_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"ORDINAL_POSITION", mysql.TypeLonglong, 10, mysql.NotNullFlag, nil, nil},
	{"POSITION_IN_UNIQUE_CONSTRAINT", mysql.TypeLonglong, 10, 0, nil, nil},
	{"REFERENCED_TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"REFERENCED_TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"REFERENCED_COLUMN_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
}

// See http://dev.mysql.com/doc/refman/5.7/en/referential-constraints-table.html
var referConstCols = []columnInfo{
	{"CONSTRAINT_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"CONSTRAINT_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"CONSTRAINT_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"UNIQUE_CONSTRAINT_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"UNIQUE_CONSTRAINT_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"UNIQUE_CONSTRAINT_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"MATCH_OPTION", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"UPDATE_RULE", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"DELETE_RULE", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"REFERENCED_TABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
}

// See http://dev.mysql.com/doc/refman/5.7/en/variables-table.html
var sessionVarCols = []columnInfo{
	{"VARIABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"VARIABLE_VALUE", mysql.TypeVarchar, 1024, 0, nil, nil},
}

// See https://dev.mysql.com/doc/refman/5.7/en/plugins-table.html
var pluginsCols = []columnInfo{
	{"PLUGIN_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"PLUGIN_VERSION", mysql.TypeVarchar, 20, 0, nil, nil},
	{"PLUGIN_STATUS", mysql.TypeVarchar, 10, 0, nil, nil},
	{"PLUGIN_TYPE", mysql.TypeVarchar, 80, 0, nil, nil},
	{"PLUGIN_TYPE_VERSION", mysql.TypeVarchar, 20, 0, nil, nil},
	{"PLUGIN_LIBRARY", mysql.TypeVarchar, 64, 0, nil, nil},
	{"PLUGIN_LIBRARY_VERSION", mysql.TypeVarchar, 20, 0, nil, nil},
	{"PLUGIN_AUTHOR", mysql.TypeVarchar, 64, 0, nil, nil},
	{"PLUGIN_DESCRIPTION", mysql.TypeLongBlob, types.UnspecifiedLength, 0, nil, nil},
	{"PLUGIN_LICENSE", mysql.TypeVarchar, 80, 0, nil, nil},
	{"LOAD_OPTION", mysql.TypeVarchar, 64, 0, nil, nil},
}

// See https://dev.mysql.com/doc/refman/5.7/en/partitions-table.html
var partitionsCols = []columnInfo{
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"PARTITION_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"SUBPARTITION_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"PARTITION_ORDINAL_POSITION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"SUBPARTITION_ORDINAL_POSITION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"PARTITION_METHOD", mysql.TypeVarchar, 18, 0, nil, nil},
	{"SUBPARTITION_METHOD", mysql.TypeVarchar, 12, 0, nil, nil},
	{"PARTITION_EXPRESSION", mysql.TypeLongBlob, types.UnspecifiedLength, 0, nil, nil},
	{"SUBPARTITION_EXPRESSION", mysql.TypeLongBlob, types.UnspecifiedLength, 0, nil, nil},
	{"PARTITION_DESCRIPTION", mysql.TypeLongBlob, types.UnspecifiedLength, 0, nil, nil},
	{"TABLE_ROWS", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AVG_ROW_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATA_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"MAX_DATA_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"INDEX_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATA_FREE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATE_TIME", mysql.TypeDatetime, 0, 0, nil, nil},
	{"UPDATE_TIME", mysql.TypeDatetime, 0, 0, nil, nil},
	{"CHECK_TIME", mysql.TypeDatetime, 0, 0, nil, nil},
	{"CHECKSUM", mysql.TypeLonglong, 21, 0, nil, nil},
	{"PARTITION_COMMENT", mysql.TypeVarchar, 80, 0, nil, nil},
	{"NODEGROUP", mysql.TypeVarchar, 12, 0, nil, nil},
	{"TABLESPACE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
}

var tableConstraintsCols = []columnInfo{
	{"CONSTRAINT_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"CONSTRAINT_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"CONSTRAINT_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"CONSTRAINT_TYPE", mysql.TypeVarchar, 64, 0, nil, nil},
}

var tableTriggersCols = []columnInfo{
	{"TRIGGER_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"TRIGGER_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TRIGGER_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"EVENT_MANIPULATION", mysql.TypeVarchar, 6, 0, nil, nil},
	{"EVENT_OBJECT_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"EVENT_OBJECT_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"EVENT_OBJECT_TABLE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"ACTION_ORDER", mysql.TypeLonglong, 4, 0, nil, nil},
	{"ACTION_CONDITION", mysql.TypeBlob, -1, 0, nil, nil},
	{"ACTION_STATEMENT", mysql.TypeBlob, -1, 0, nil, nil},
	{"ACTION_ORIENTATION", mysql.TypeVarchar, 9, 0, nil, nil},
	{"ACTION_TIMING", mysql.TypeVarchar, 6, 0, nil, nil},
	{"ACTION_REFERENCE_OLD_TABLE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"ACTION_REFERENCE_NEW_TABLE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"ACTION_REFERENCE_OLD_ROW", mysql.TypeVarchar, 3, 0, nil, nil},
	{"ACTION_REFERENCE_NEW_ROW", mysql.TypeVarchar, 3, 0, nil, nil},
	{"CREATED", mysql.TypeDatetime, 2, 0, nil, nil},
	{"SQL_MODE", mysql.TypeVarchar, 8192, 0, nil, nil},
	{"DEFINER", mysql.TypeVarchar, 77, 0, nil, nil},
	{"CHARACTER_SET_CLIENT", mysql.TypeVarchar, 32, 0, nil, nil},
	{"COLLATION_CONNECTION", mysql.TypeVarchar, 32, 0, nil, nil},
	{"DATABASE_COLLATION", mysql.TypeVarchar, 32, 0, nil, nil},
}

var tableUserPrivilegesCols = []columnInfo{
	{"GRANTEE", mysql.TypeVarchar, 81, 0, nil, nil},
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, 0, nil, nil},
	{"PRIVILEGE_TYPE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"IS_GRANTABLE", mysql.TypeVarchar, 3, 0, nil, nil},
}

var tableSchemaPrivilegesCols = []columnInfo{
	{"GRANTEE", mysql.TypeVarchar, 81, mysql.NotNullFlag, nil, nil},
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"PRIVILEGE_TYPE", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"IS_GRANTABLE", mysql.TypeVarchar, 3, mysql.NotNullFlag, nil, nil},
}

var tableTablePrivilegesCols = []columnInfo{
	{"GRANTEE", mysql.TypeVarchar, 81, mysql.NotNullFlag, nil, nil},
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"PRIVILEGE_TYPE", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"IS_GRANTABLE", mysql.TypeVarchar, 3, mysql.NotNullFlag, nil, nil},
}

var tableColumnPrivilegesCols = []columnInfo{
	{"GRANTEE", mysql.TypeVarchar, 81, mysql.NotNullFlag, nil, nil},
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"COLUMN_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"PRIVILEGE_TYPE", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"IS_GRANTABLE", mysql.TypeVarchar, 3, mysql.NotNullFlag, nil, nil},
}

var tableEnginesCols = []columnInfo{
	{"ENGINE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"SUPPORT", mysql.TypeVarchar, 8, 0, nil, nil},
	{"COMMENT", mysql.TypeVarchar, 80, 0, nil, nil},
	{"TRANSACTIONS", mysql.TypeVarchar, 3, 0, nil, nil},
	{"XA", mysql.TypeVarchar, 3, 0, nil, nil},
	{"SAVEPOINTS", mysql.TypeVarchar, 3, 0, nil, nil},
}

var tableViewsCols = []columnInfo{
	{"TABLE_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"VIEW_DEFINITION", mysql.TypeLongBlob, 0, mysql.NotNullFlag, nil, nil},
	{"CHECK_OPTION", mysql.TypeVarchar, 8, mysql.NotNullFlag, nil, nil},
	{"IS_UPDATABLE", mysql.TypeVarchar, 3, mysql.NotNullFlag, nil, nil},
	{"DEFINER", mysql.TypeVarchar, 77, mysql.NotNullFlag, nil, nil},
	{"SECURITY_TYPE", mysql.TypeVarchar, 7, mysql.NotNullFlag, nil, nil},
	{"CHARACTER_SET_CLIENT", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
	{"COLLATION_CONNECTION", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
}

var tableRoutinesCols = []columnInfo{
	{"SPECIFIC_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"ROUTINE_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"ROUTINE_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"ROUTINE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"ROUTINE_TYPE", mysql.TypeVarchar, 9, mysql.NotNullFlag, nil, nil},
	{"DATA_TYPE", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"CHARACTER_MAXIMUM_LENGTH", mysql.TypeLong, 21, 0, nil, nil},
	{"CHARACTER_OCTET_LENGTH", mysql.TypeLong, 21, 0, nil, nil},
	{"NUMERIC_PRECISION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"NUMERIC_SCALE", mysql.TypeLong, 21, 0, nil, nil},
	{"DATETIME_PRECISION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"COLLATION_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DTD_IDENTIFIER", mysql.TypeLongBlob, 0, 0, nil, nil},
	{"ROUTINE_BODY", mysql.TypeVarchar, 8, mysql.NotNullFlag, nil, nil},
	{"ROUTINE_DEFINITION", mysql.TypeLongBlob, 0, 0, nil, nil},
	{"EXTERNAL_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"EXTERNAL_LANGUAGE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"PARAMETER_STYLE", mysql.TypeVarchar, 8, mysql.NotNullFlag, nil, nil},
	{"IS_DETERMINISTIC", mysql.TypeVarchar, 3, mysql.NotNullFlag, nil, nil},
	{"SQL_DATA_ACCESS", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"SQL_PATH", mysql.TypeVarchar, 64, 0, nil, nil},
	{"SECURITY_TYPE", mysql.TypeVarchar, 7, mysql.NotNullFlag, nil, nil},
	{"CREATED", mysql.TypeDatetime, 0, mysql.NotNullFlag, "0000-00-00 00:00:00", nil},
	{"LAST_ALTERED", mysql.TypeDatetime, 0, mysql.NotNullFlag, "0000-00-00 00:00:00", nil},
	{"SQL_MODE", mysql.TypeVarchar, 8192, mysql.NotNullFlag, nil, nil},
	{"ROUTINE_COMMENT", mysql.TypeLongBlob, 0, 0, nil, nil},
	{"DEFINER", mysql.TypeVarchar, 77, mysql.NotNullFlag, nil, nil},
	{"CHARACTER_SET_CLIENT", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
	{"COLLATION_CONNECTION", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
	{"DATABASE_COLLATION", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
}

var tableParametersCols = []columnInfo{
	{"SPECIFIC_CATALOG", mysql.TypeVarchar, 512, mysql.NotNullFlag, nil, nil},
	{"SPECIFIC_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"SPECIFIC_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"ORDINAL_POSITION", mysql.TypeVarchar, 21, mysql.NotNullFlag, nil, nil},
	{"PARAMETER_MODE", mysql.TypeVarchar, 5, 0, nil, nil},
	{"PARAMETER_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DATA_TYPE", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"CHARACTER_MAXIMUM_LENGTH", mysql.TypeVarchar, 21, 0, nil, nil},
	{"CHARACTER_OCTET_LENGTH", mysql.TypeVarchar, 21, 0, nil, nil},
	{"NUMERIC_PRECISION", mysql.TypeVarchar, 21, 0, nil, nil},
	{"NUMERIC_SCALE", mysql.TypeVarchar, 21, 0, nil, nil},
	{"DATETIME_PRECISION", mysql.TypeVarchar, 21, 0, nil, nil},
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"COLLATION_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DTD_IDENTIFIER", mysql.TypeLongBlob, 0, mysql.NotNullFlag, nil, nil},
	{"ROUTINE_TYPE", mysql.TypeVarchar, 9, mysql.NotNullFlag, nil, nil},
}

var tableEventsCols = []columnInfo{
	{"EVENT_CATALOG", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"EVENT_SCHEMA", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"EVENT_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"DEFINER", mysql.TypeVarchar, 77, mysql.NotNullFlag, nil, nil},
	{"TIME_ZONE", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"EVENT_BODY", mysql.TypeVarchar, 8, mysql.NotNullFlag, nil, nil},
	{"EVENT_DEFINITION", mysql.TypeLongBlob, 0, 0, nil, nil},
	{"EVENT_TYPE", mysql.TypeVarchar, 9, mysql.NotNullFlag, nil, nil},
	{"EXECUTE_AT", mysql.TypeDatetime, 0, 0, nil, nil},
	{"INTERVAL_VALUE", mysql.TypeVarchar, 256, 0, nil, nil},
	{"INTERVAL_FIELD", mysql.TypeVarchar, 18, 0, nil, nil},
	{"SQL_MODE", mysql.TypeVarchar, 8192, mysql.NotNullFlag, nil, nil},
	{"STARTS", mysql.TypeDatetime, 0, 0, nil, nil},
	{"ENDS", mysql.TypeDatetime, 0, 0, nil, nil},
	{"STATUS", mysql.TypeVarchar, 18, mysql.NotNullFlag, nil, nil},
	{"ON_COMPLETION", mysql.TypeVarchar, 12, mysql.NotNullFlag, nil, nil},
	{"CREATED", mysql.TypeDatetime, 0, mysql.NotNullFlag, "0000-00-00 00:00:00", nil},
	{"LAST_ALTERED", mysql.TypeDatetime, 0, mysql.NotNullFlag, "0000-00-00 00:00:00", nil},
	{"LAST_EXECUTED", mysql.TypeDatetime, 0, 0, nil, nil},
	{"EVENT_COMMENT", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"ORIGINATOR", mysql.TypeLong, 10, mysql.NotNullFlag, 0, nil},
	{"CHARACTER_SET_CLIENT", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
	{"COLLATION_CONNECTION", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
	{"DATABASE_COLLATION", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
}

var tableGlobalStatusCols = []columnInfo{
	{"VARIABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"VARIABLE_VALUE", mysql.TypeVarchar, 1024, 0, nil, nil},
}

var tableGlobalVariablesCols = []columnInfo{
	{"VARIABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"VARIABLE_VALUE", mysql.TypeVarchar, 1024, 0, nil, nil},
}

var tableSessionStatusCols = []columnInfo{
	{"VARIABLE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, nil, nil},
	{"VARIABLE_VALUE", mysql.TypeVarchar, 1024, 0, nil, nil},
}

var tableOptimizerTraceCols = []columnInfo{
	{"QUERY", mysql.TypeLongBlob, 0, mysql.NotNullFlag, "", nil},
	{"TRACE", mysql.TypeLongBlob, 0, mysql.NotNullFlag, "", nil},
	{"MISSING_BYTES_BEYOND_MAX_MEM_SIZE", mysql.TypeShort, 20, mysql.NotNullFlag, 0, nil},
	{"INSUFFICIENT_PRIVILEGES", mysql.TypeTiny, 1, mysql.NotNullFlag, 0, nil},
}

var tableTableSpacesCols = []columnInfo{
	{"TABLESPACE_NAME", mysql.TypeVarchar, 64, mysql.NotNullFlag, "", nil},
	{"ENGINE", mysql.TypeVarchar, 64, mysql.NotNullFlag, "", nil},
	{"TABLESPACE_TYPE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"LOGFILE_GROUP_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"EXTENT_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AUTOEXTEND_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"MAXIMUM_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"NODEGROUP_ID", mysql.TypeLonglong, 21, 0, nil, nil},
	{"TABLESPACE_COMMENT", mysql.TypeVarchar, 2048, 0, nil, nil},
}

var tableCollationCharacterSetApplicabilityCols = []columnInfo{
	{"COLLATION_NAME", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
	{"CHARACTER_SET_NAME", mysql.TypeVarchar, 32, mysql.NotNullFlag, nil, nil},
}

var tableProcesslistCols = []columnInfo{
	{"ID", mysql.TypeLonglong, 21, mysql.NotNullFlag, 0, nil},
	{"USER", mysql.TypeVarchar, 16, mysql.NotNullFlag, "", nil},
	{"HOST", mysql.TypeVarchar, 64, mysql.NotNullFlag, "", nil},
	{"DB", mysql.TypeVarchar, 64, mysql.NotNullFlag, "", nil},
	{"COMMAND", mysql.TypeVarchar, 16, mysql.NotNullFlag, "", nil},
	{"TIME", mysql.TypeLong, 7, mysql.NotNullFlag, 0, nil},
	{"STATE", mysql.TypeVarchar, 7, 0, nil, nil},
	{"Info", mysql.TypeString, 512, 0, nil, nil},
}

func dataForCharacterSets() (records [][]types.Datum) {

	charsets := charset.GetAllCharsets()

	for _, charset := range charsets {

		records = append(records,
			types.MakeDatums(charset.Name, charset.DefaultCollation, charset.Desc, charset.Maxlen),
		)

	}

	return records

}

func dataForCollations() (records [][]types.Datum) {

	collations := charset.GetCollations()

	for _, collation := range collations {

		isDefault := ""
		if collation.IsDefault {
			isDefault = "Yes"
		}

		records = append(records,
			types.MakeDatums(collation.Name, collation.CharsetName, collation.ID, isDefault, "Yes", 1),
		)

	}

	return records

}

func dataForCollationCharacterSetApplicability() (records [][]types.Datum) {

	collations := charset.GetCollations()

	for _, collation := range collations {

		records = append(records,
			types.MakeDatums(collation.Name, collation.CharsetName),
		)

	}

	return records

}

func dataForSessionVar(ctx sessionctx.Context) (records [][]types.Datum, err error) {
	sessionVars := ctx.GetSessionVars()
	for _, v := range variable.SysVars {
		var value string
		value, err = variable.GetSessionSystemVar(sessionVars, v.Name)
		if err != nil {
			return nil, errors.Trace(err)
		}
		row := types.MakeDatums(v.Name, value)
		records = append(records, row)
	}
	return
}

func dataForUserPrivileges(ctx sessionctx.Context) [][]types.Datum {
	pm := privilege.GetPrivilegeManager(ctx)
	return pm.UserPrivilegesTable()
}

func dataForProcesslist(ctx sessionctx.Context) [][]types.Datum {
	sm := ctx.GetSessionManager()
	if sm == nil {
		return nil
	}

	var records [][]types.Datum
	pl := sm.ShowProcessList()
	for _, pi := range pl {
		var t uint64
		if len(pi.Info) != 0 {
			t = uint64(time.Since(pi.Time) / time.Second)
		}
		record := types.MakeDatums(
			pi.ID,
			pi.User,
			pi.Host,
			pi.DB,
			pi.Command,
			t,
			fmt.Sprintf("%d", pi.State),
			pi.Info,
		)
		records = append(records, record)
	}
	return records
}

func dataForEngines() (records [][]types.Datum) {
	records = append(records,
		types.MakeDatums("InnoDB", "DEFAULT", "Supports transactions, row-level locking, and foreign keys", "YES", "YES", "YES"),
		types.MakeDatums("CSV", "YES", "CSV storage engine", "NO", "NO", "NO"),
		types.MakeDatums("MRG_MYISAM", "YES", "Collection of identical MyISAM tables", "NO", "NO", "NO"),
		types.MakeDatums("BLACKHOLE", "YES", "/dev/null storage engine (anything you write to it disappears)", "NO", "NO", "NO"),
		types.MakeDatums("MyISAM", "YES", "MyISAM storage engine", "NO", "NO", "NO"),
		types.MakeDatums("MEMORY", "YES", "Hash based, stored in memory, useful for temporary tables", "NO", "NO", "NO"),
		types.MakeDatums("ARCHIVE", "YES", "Archive storage engine", "NO", "NO", "NO"),
		types.MakeDatums("FEDERATED", "NO", "Federated MySQL storage engine", nil, nil, nil),
		types.MakeDatums("PERFORMANCE_SCHEMA", "YES", "Performance Schema", "NO", "NO", "NO"),
	)
	return records
}

var filesCols = []columnInfo{
	{"FILE_ID", mysql.TypeLonglong, 4, 0, nil, nil},
	{"FILE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"FILE_TYPE", mysql.TypeVarchar, 20, 0, nil, nil},
	{"TABLESPACE_NAME", mysql.TypeVarchar, 20, 0, nil, nil},
	{"TABLE_CATALOG", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_SCHEMA", mysql.TypeVarchar, 64, 0, nil, nil},
	{"TABLE_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"LOGFILE_GROUP_NAME", mysql.TypeVarchar, 64, 0, nil, nil},
	{"LOGFILE_GROUP_NUMBER", mysql.TypeLonglong, 32, 0, nil, nil},
	{"ENGINE", mysql.TypeVarchar, 64, 0, nil, nil},
	{"FULLTEXT_KEYS", mysql.TypeVarchar, 64, 0, nil, nil},
	{"DELETED_ROWS", mysql.TypeLonglong, 4, 0, nil, nil},
	{"UPDATE_COUNT", mysql.TypeLonglong, 4, 0, nil, nil},
	{"FREE_EXTENTS", mysql.TypeLonglong, 4, 0, nil, nil},
	{"TOTAL_EXTENTS", mysql.TypeLonglong, 4, 0, nil, nil},
	{"EXTENT_SIZE", mysql.TypeLonglong, 4, 0, nil, nil},
	{"INITIAL_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"MAXIMUM_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AUTOEXTEND_SIZE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATION_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"LAST_UPDATE_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"LAST_ACCESS_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"RECOVER_TIME", mysql.TypeLonglong, 4, 0, nil, nil},
	{"TRANSACTION_COUNTER", mysql.TypeLonglong, 4, 0, nil, nil},
	{"VERSION", mysql.TypeLonglong, 21, 0, nil, nil},
	{"ROW_FORMAT", mysql.TypeVarchar, 21, 0, nil, nil},
	{"TABLE_ROWS", mysql.TypeLonglong, 21, 0, nil, nil},
	{"AVG_ROW_LENGTH", mysql.TypeLonglong, 21, 0, nil, nil},
	{"DATA_FREE", mysql.TypeLonglong, 21, 0, nil, nil},
	{"CREATE_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"UPDATE_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"CHECK_TIME", mysql.TypeDatetime, -1, 0, nil, nil},
	{"CHECKSUM", mysql.TypeLonglong, 21, 0, nil, nil},
	{"STATUS", mysql.TypeVarchar, 20, 0, nil, nil},
	{"EXTRA", mysql.TypeVarchar, 255, 0, nil, nil},
}

func dataForSchemata(schemas []*model.DBInfo) [][]types.Datum {

	var rows [][]types.Datum

	for _, schema := range schemas {

		charset := mysql.DefaultCharset
		collation := mysql.DefaultCollationName

		if len(schema.Charset) > 0 {
			charset = schema.Charset // Overwrite default
		}

		if len(schema.Collate) > 0 {
			collation = schema.Collate // Overwrite default
		}

		record := types.MakeDatums(
			catalogVal,    // CATALOG_NAME
			schema.Name.O, // SCHEMA_NAME
			charset,       // DEFAULT_CHARACTER_SET_NAME
			collation,     // DEFAULT_COLLATION_NAME
			nil,
		)
		rows = append(rows, record)
	}
	return rows
}

func getRowCountAllTable(ctx sessionctx.Context) (map[int64]uint64, error) {
	rows, _, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, "select table_id, count from mysql.stats_meta")
	if err != nil {
		return nil, errors.Trace(err)
	}
	rowCountMap := make(map[int64]uint64, len(rows))
	for _, row := range rows {
		tableID := row.GetInt64(0)
		rowCnt := row.GetUint64(1)
		rowCountMap[tableID] = rowCnt
	}
	return rowCountMap, nil
}

type tableHistID struct {
	tableID int64
	histID  int64
}

func getColLengthAllTables(ctx sessionctx.Context) (map[tableHistID]uint64, error) {
	rows, _, err := ctx.(sqlexec.RestrictedSQLExecutor).ExecRestrictedSQL(ctx, "select table_id, hist_id, tot_col_size from mysql.stats_histograms where is_index = 0")
	if err != nil {
		return nil, errors.Trace(err)
	}
	colLengthMap := make(map[tableHistID]uint64, len(rows))
	for _, row := range rows {
		tableID := row.GetInt64(0)
		histID := row.GetInt64(1)
		totalSize := row.GetInt64(2)
		if totalSize < 0 {
			totalSize = 0
		}
		colLengthMap[tableHistID{tableID: tableID, histID: histID}] = uint64(totalSize)
	}
	return colLengthMap, nil
}

func getDataAndIndexLength(info *model.TableInfo, rowCount uint64, columnLengthMap map[tableHistID]uint64) (uint64, uint64) {
	columnLength := make(map[string]uint64)
	for _, col := range info.Columns {
		if col.State != model.StatePublic {
			continue
		}
		length := col.FieldType.StorageLength()
		if length != types.VarStorageLen {
			columnLength[col.Name.L] = rowCount * uint64(length)
		} else {
			length := columnLengthMap[tableHistID{tableID: info.ID, histID: col.ID}]
			columnLength[col.Name.L] = length
		}
	}
	dataLength, indexLength := uint64(0), uint64(0)
	for _, length := range columnLength {
		dataLength += length
	}
	for _, idx := range info.Indices {
		if idx.State != model.StatePublic {
			continue
		}
		for _, col := range idx.Columns {
			if col.Length == types.UnspecifiedLength {
				indexLength += columnLength[col.Name.L]
			} else {
				indexLength += rowCount * uint64(col.Length)
			}
		}
	}
	return dataLength, indexLength
}

type statsCache struct {
	mu         sync.Mutex
	loading    bool
	modifyTime time.Time
	tableRows  map[int64]uint64
	colLength  map[tableHistID]uint64
}

var tableStatsCache = &statsCache{}

// TableStatsCacheExpiry is the expiry time for table stats cache.
var TableStatsCacheExpiry = 3 * time.Second

func (c *statsCache) setLoading(loading bool) {
	c.mu.Lock()
	c.loading = loading
	c.mu.Unlock()
}

func (c *statsCache) get(ctx sessionctx.Context) (map[int64]uint64, map[tableHistID]uint64, error) {
	c.mu.Lock()
	if time.Now().Sub(c.modifyTime) < TableStatsCacheExpiry || c.loading {
		tableRows, colLength := c.tableRows, c.colLength
		c.mu.Unlock()
		return tableRows, colLength, nil
	}
	c.loading = true
	c.mu.Unlock()

	tableRows, err := getRowCountAllTable(ctx)
	if err != nil {
		c.setLoading(false)
		return nil, nil, errors.Trace(err)
	}
	colLength, err := getColLengthAllTables(ctx)
	if err != nil {
		c.setLoading(false)
		return nil, nil, errors.Trace(err)
	}

	c.mu.Lock()
	c.loading = false
	c.tableRows = tableRows
	c.colLength = colLength
	c.modifyTime = time.Now()
	c.mu.Unlock()
	return tableRows, colLength, nil
}

func getAutoIncrementID(ctx sessionctx.Context, schema *model.DBInfo, tblInfo *model.TableInfo) (int64, error) {
	hasAutoIncID := false
	for _, col := range tblInfo.Cols() {
		if mysql.HasAutoIncrementFlag(col.Flag) {
			hasAutoIncID = true
			break
		}
	}
	autoIncID := tblInfo.AutoIncID
	if hasAutoIncID {
		is := ctx.GetSessionVars().TxnCtx.InfoSchema.(InfoSchema)
		tbl, err := is.TableByName(schema.Name, tblInfo.Name)
		if err != nil {
			return 0, errors.Trace(err)
		}
		autoIncID, err = tbl.Allocator(ctx).NextGlobalAutoID(tblInfo.ID)
		if err != nil {
			return 0, errors.Trace(err)
		}
	}
	return autoIncID, nil
}

func dataForTables(ctx sessionctx.Context, schemas []*model.DBInfo) ([][]types.Datum, error) {
	tableRowsMap, colLengthMap, err := tableStatsCache.get(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}

	checker := privilege.GetPrivilegeManager(ctx)

	var rows [][]types.Datum
	createTimeTp := tablesCols[15].tp
	for _, schema := range schemas {
		for _, table := range schema.Tables {
			collation := table.Collate
			if collation == "" {
				collation = mysql.DefaultCollationName
			}
			createTime := types.Time{
				Time: types.FromGoTime(table.GetUpdateTime()),
				Type: createTimeTp,
			}

			if checker != nil && !checker.RequestVerification(schema.Name.L, table.Name.L, "", mysql.AllPrivMask) {
				continue
			}

			autoIncID, err := getAutoIncrementID(ctx, schema, table)
			if err != nil {
				return nil, errors.Trace(err)
			}
			rowCount := tableRowsMap[table.ID]
			dataLength, indexLength := getDataAndIndexLength(table, rowCount, colLengthMap)
			avgRowLength := uint64(0)
			if rowCount != 0 {
				avgRowLength = dataLength / rowCount
			}
			record := types.MakeDatums(
				catalogVal,    // TABLE_CATALOG
				schema.Name.O, // TABLE_SCHEMA
				table.Name.O,  // TABLE_NAME
				"BASE TABLE",  // TABLE_TYPE
				"InnoDB",      // ENGINE
				uint64(10),    // VERSION
				"Compact",     // ROW_FORMAT
				rowCount,      // TABLE_ROWS
				avgRowLength,  // AVG_ROW_LENGTH
				dataLength,    // DATA_LENGTH
				uint64(0),     // MAX_DATA_LENGTH
				indexLength,   // INDEX_LENGTH
				uint64(0),     // DATA_FREE
				autoIncID,     // AUTO_INCREMENT
				createTime,    // CREATE_TIME
				nil,           // UPDATE_TIME
				nil,           // CHECK_TIME
				collation,     // TABLE_COLLATION
				nil,           // CHECKSUM
				"",            // CREATE_OPTIONS
				table.Comment, // TABLE_COMMENT
			)
			rows = append(rows, record)
		}
	}
	return rows, nil
}

func dataForColumns(ctx sessionctx.Context, schemas []*model.DBInfo) [][]types.Datum {
	checker := privilege.GetPrivilegeManager(ctx)
	var rows [][]types.Datum
	for _, schema := range schemas {
		for _, table := range schema.Tables {
			if checker != nil && !checker.RequestVerification(schema.Name.L, table.Name.L, "", mysql.AllPrivMask) {
				continue
			}

			rs := dataForColumnsInTable(schema, table)
			rows = append(rows, rs...)
		}
	}
	return rows
}

func dataForColumnsInTable(schema *model.DBInfo, tbl *model.TableInfo) [][]types.Datum {
	var rows [][]types.Datum
	for i, col := range tbl.Columns {
		var charMaxLen, charOctLen, numericPrecision, numericScale, datetimePrecision interface{}
		colLen, decimal := col.Flen, col.Decimal
		defaultFlen, defaultDecimal := mysql.GetDefaultFieldLengthAndDecimal(col.Tp)
		if decimal == types.UnspecifiedLength {
			decimal = defaultDecimal
		}
		if colLen == types.UnspecifiedLength {
			colLen = defaultFlen
		}
		if col.Tp == mysql.TypeSet {
			// Example: In MySQL set('a','bc','def','ghij') has length 13, because
			// len('a')+len('bc')+len('def')+len('ghij')+len(ThreeComma)=13
			// Reference link: https://bugs.mysql.com/bug.php?id=22613
			colLen = 0
			for _, ele := range col.Elems {
				colLen += len(ele)
			}
			if len(col.Elems) != 0 {
				colLen += (len(col.Elems) - 1)
			}
			charMaxLen = colLen
			charOctLen = colLen
		} else if col.Tp == mysql.TypeEnum {
			// Example: In MySQL enum('a', 'ab', 'cdef') has length 4, because
			// the longest string in the enum is 'cdef'
			// Reference link: https://bugs.mysql.com/bug.php?id=22613
			colLen = 0
			for _, ele := range col.Elems {
				if len(ele) > colLen {
					colLen = len(ele)
				}
			}
			charMaxLen = colLen
			charOctLen = colLen
		} else if types.IsString(col.Tp) {
			charMaxLen = colLen
			charOctLen = colLen
		} else if types.IsTypeFractionable(col.Tp) {
			datetimePrecision = decimal
		} else if types.IsTypeNumeric(col.Tp) {
			numericPrecision = colLen
			if col.Tp != mysql.TypeFloat && col.Tp != mysql.TypeDouble {
				numericScale = decimal
			} else if decimal != -1 {
				numericScale = decimal
			}
		}
		columnType := col.FieldType.InfoSchemaStr()
		columnDesc := table.NewColDesc(table.ToColumn(col))
		var columnDefault interface{}
		if columnDesc.DefaultValue != nil {
			columnDefault = fmt.Sprintf("%v", columnDesc.DefaultValue)
		}
		record := types.MakeDatums(
			catalogVal,                           // TABLE_CATALOG
			schema.Name.O,                        // TABLE_SCHEMA
			tbl.Name.O,                           // TABLE_NAME
			col.Name.O,                           // COLUMN_NAME
			i+1,                                  // ORIGINAL_POSITION
			columnDefault,                        // COLUMN_DEFAULT
			columnDesc.Null,                      // IS_NULLABLE
			types.TypeToStr(col.Tp, col.Charset), // DATA_TYPE
			charMaxLen,                           // CHARACTER_MAXIMUM_LENGTH
			charOctLen,                           // CHARACTER_OCTET_LENGTH
			numericPrecision,                     // NUMERIC_PRECISION
			numericScale,                         // NUMERIC_SCALE
			datetimePrecision,                    // DATETIME_PRECISION
			col.Charset,                          // CHARACTER_SET_NAME
			col.Collate,                          // COLLATION_NAME
			columnType,                           // COLUMN_TYPE
			columnDesc.Key,                       // COLUMN_KEY
			columnDesc.Extra,                     // EXTRA
			"select,insert,update,references",    // PRIVILEGES
			columnDesc.Comment,                   // COLUMN_COMMENT
			col.GeneratedExprString,              // GENERATION_EXPRESSION
		)
		// In mysql, 'character_set_name' and 'collation_name' are setted to null when column type is non-varchar or non-blob in information_schema.
		if col.Tp != mysql.TypeVarchar && col.Tp != mysql.TypeBlob {
			record[13].SetNull()
			record[14].SetNull()
		}

		rows = append(rows, record)
	}
	return rows
}

func dataForStatistics(schemas []*model.DBInfo) [][]types.Datum {
	var rows [][]types.Datum
	for _, schema := range schemas {
		for _, table := range schema.Tables {
			rs := dataForStatisticsInTable(schema, table)
			rows = append(rows, rs...)
		}
	}
	return rows
}

func dataForStatisticsInTable(schema *model.DBInfo, table *model.TableInfo) [][]types.Datum {
	var rows [][]types.Datum
	if table.PKIsHandle {
		for _, col := range table.Columns {
			if mysql.HasPriKeyFlag(col.Flag) {
				record := types.MakeDatums(
					catalogVal,    // TABLE_CATALOG
					schema.Name.O, // TABLE_SCHEMA
					table.Name.O,  // TABLE_NAME
					"0",           // NON_UNIQUE
					schema.Name.O, // INDEX_SCHEMA
					"PRIMARY",     // INDEX_NAME
					1,             // SEQ_IN_INDEX
					col.Name.O,    // COLUMN_NAME
					"A",           // COLLATION
					0,             // CARDINALITY
					nil,           // SUB_PART
					nil,           // PACKED
					"",            // NULLABLE
					"BTREE",       // INDEX_TYPE
					"",            // COMMENT
					"",            // INDEX_COMMENT
				)
				rows = append(rows, record)
			}
		}
	}
	nameToCol := make(map[string]*model.ColumnInfo, len(table.Columns))
	for _, c := range table.Columns {
		nameToCol[c.Name.L] = c
	}
	for _, index := range table.Indices {
		nonUnique := "1"
		if index.Unique {
			nonUnique = "0"
		}
		for i, key := range index.Columns {
			col := nameToCol[key.Name.L]
			nullable := "YES"
			if mysql.HasNotNullFlag(col.Flag) {
				nullable = ""
			}
			record := types.MakeDatums(
				catalogVal,    // TABLE_CATALOG
				schema.Name.O, // TABLE_SCHEMA
				table.Name.O,  // TABLE_NAME
				nonUnique,     // NON_UNIQUE
				schema.Name.O, // INDEX_SCHEMA
				index.Name.O,  // INDEX_NAME
				i+1,           // SEQ_IN_INDEX
				key.Name.O,    // COLUMN_NAME
				"A",           // COLLATION
				0,             // CARDINALITY
				nil,           // SUB_PART
				nil,           // PACKED
				nullable,      // NULLABLE
				"BTREE",       // INDEX_TYPE
				"",            // COMMENT
				"",            // INDEX_COMMENT
			)
			rows = append(rows, record)
		}
	}
	return rows
}

const (
	primaryKeyType    = "PRIMARY KEY"
	primaryConstraint = "PRIMARY"
	uniqueKeyType     = "UNIQUE"
)

// dataForTableConstraints constructs data for table information_schema.constraints.See https://dev.mysql.com/doc/refman/5.7/en/table-constraints-table.html
func dataForTableConstraints(schemas []*model.DBInfo) [][]types.Datum {
	var rows [][]types.Datum
	for _, schema := range schemas {
		for _, tbl := range schema.Tables {
			if tbl.PKIsHandle {
				record := types.MakeDatums(
					catalogVal,           // CONSTRAINT_CATALOG
					schema.Name.O,        // CONSTRAINT_SCHEMA
					mysql.PrimaryKeyName, // CONSTRAINT_NAME
					schema.Name.O,        // TABLE_SCHEMA
					tbl.Name.O,           // TABLE_NAME
					primaryKeyType,       // CONSTRAINT_TYPE
				)
				rows = append(rows, record)
			}

			for _, idx := range tbl.Indices {
				var cname, ctype string
				if idx.Primary {
					cname = mysql.PrimaryKeyName
					ctype = primaryKeyType
				} else if idx.Unique {
					cname = idx.Name.O
					ctype = uniqueKeyType
				} else {
					// The index has no constriant.
					continue
				}
				record := types.MakeDatums(
					catalogVal,    // CONSTRAINT_CATALOG
					schema.Name.O, // CONSTRAINT_SCHEMA
					cname,         // CONSTRAINT_NAME
					schema.Name.O, // TABLE_SCHEMA
					tbl.Name.O,    // TABLE_NAME
					ctype,         // CONSTRAINT_TYPE
				)
				rows = append(rows, record)
			}
		}
	}
	return rows
}

// dataForPseudoProfiling returns pseudo data for table profiling when system variable `profiling` is set to `ON`.
func dataForPseudoProfiling() [][]types.Datum {
	var rows [][]types.Datum
	row := types.MakeDatums(
		0,                      // QUERY_ID
		0,                      // SEQ
		"",                     // STATE
		types.NewDecFromInt(0), // DURATION
		types.NewDecFromInt(0), // CPU_USER
		types.NewDecFromInt(0), // CPU_SYSTEM
		0,                      // CONTEXT_VOLUNTARY
		0,                      // CONTEXT_INVOLUNTARY
		0,                      // BLOCK_OPS_IN
		0,                      // BLOCK_OPS_OUT
		0,                      // MESSAGES_SENT
		0,                      // MESSAGES_RECEIVED
		0,                      // PAGE_FAULTS_MAJOR
		0,                      // PAGE_FAULTS_MINOR
		0,                      // SWAPS
		"",                     // SOURCE_FUNCTION
		"",                     // SOURCE_FILE
		0,                      // SOURCE_LINE
	)
	rows = append(rows, row)
	return rows
}

func dataForKeyColumnUsage(schemas []*model.DBInfo) [][]types.Datum {
	rows := make([][]types.Datum, 0, len(schemas)) // The capacity is not accurate, but it is not a big problem.
	for _, schema := range schemas {
		for _, table := range schema.Tables {
			rs := keyColumnUsageInTable(schema, table)
			rows = append(rows, rs...)
		}
	}
	return rows
}

func keyColumnUsageInTable(schema *model.DBInfo, table *model.TableInfo) [][]types.Datum {
	var rows [][]types.Datum
	if table.PKIsHandle {
		for _, col := range table.Columns {
			if mysql.HasPriKeyFlag(col.Flag) {
				record := types.MakeDatums(
					catalogVal,        // CONSTRAINT_CATALOG
					schema.Name.O,     // CONSTRAINT_SCHEMA
					primaryConstraint, // CONSTRAINT_NAME
					catalogVal,        // TABLE_CATALOG
					schema.Name.O,     // TABLE_SCHEMA
					table.Name.O,      // TABLE_NAME
					col.Name.O,        // COLUMN_NAME
					1,                 // ORDINAL_POSITION
					1,                 // POSITION_IN_UNIQUE_CONSTRAINT
					nil,               // REFERENCED_TABLE_SCHEMA
					nil,               // REFERENCED_TABLE_NAME
					nil,               // REFERENCED_COLUMN_NAME
				)
				rows = append(rows, record)
				break
			}
		}
	}
	nameToCol := make(map[string]*model.ColumnInfo, len(table.Columns))
	for _, c := range table.Columns {
		nameToCol[c.Name.L] = c
	}
	for _, index := range table.Indices {
		var idxName string
		if index.Primary {
			idxName = primaryConstraint
		} else if index.Unique {
			idxName = index.Name.O
		} else {
			// Only handle unique/primary key
			continue
		}
		for i, key := range index.Columns {
			col := nameToCol[key.Name.L]
			record := types.MakeDatums(
				catalogVal,    // CONSTRAINT_CATALOG
				schema.Name.O, // CONSTRAINT_SCHEMA
				idxName,       // CONSTRAINT_NAME
				catalogVal,    // TABLE_CATALOG
				schema.Name.O, // TABLE_SCHEMA
				table.Name.O,  // TABLE_NAME
				col.Name.O,    // COLUMN_NAME
				i+1,           // ORDINAL_POSITION,
				nil,           // POSITION_IN_UNIQUE_CONSTRAINT
				nil,           // REFERENCED_TABLE_SCHEMA
				nil,           // REFERENCED_TABLE_NAME
				nil,           // REFERENCED_COLUMN_NAME
			)
			rows = append(rows, record)
		}
	}
	for _, fk := range table.ForeignKeys {
		fkRefCol := ""
		if len(fk.RefCols) > 0 {
			fkRefCol = fk.RefCols[0].O
		}
		for i, key := range fk.Cols {
			col := nameToCol[key.L]
			record := types.MakeDatums(
				catalogVal,    // CONSTRAINT_CATALOG
				schema.Name.O, // CONSTRAINT_SCHEMA
				fk.Name.O,     // CONSTRAINT_NAME
				catalogVal,    // TABLE_CATALOG
				schema.Name.O, // TABLE_SCHEMA
				table.Name.O,  // TABLE_NAME
				col.Name.O,    // COLUMN_NAME
				i+1,           // ORDINAL_POSITION,
				1,             // POSITION_IN_UNIQUE_CONSTRAINT
				schema.Name.O, // REFERENCED_TABLE_SCHEMA
				fk.RefTable.O, // REFERENCED_TABLE_NAME
				fkRefCol,      // REFERENCED_COLUMN_NAME
			)
			rows = append(rows, record)
		}
	}
	return rows
}

var tableNameToColumns = map[string][]columnInfo{
	tableSchemata:                           schemataCols,
	tableTables:                             tablesCols,
	tableColumns:                            columnsCols,
	tableStatistics:                         statisticsCols,
	tableCharacterSets:                      charsetCols,
	tableCollations:                         collationsCols,
	tableFiles:                              filesCols,
	tableProfiling:                          profilingCols,
	tablePartitions:                         partitionsCols,
	tableKeyColumm:                          keyColumnUsageCols,
	tableReferConst:                         referConstCols,
	tableSessionVar:                         sessionVarCols,
	tablePlugins:                            pluginsCols,
	tableConstraints:                        tableConstraintsCols,
	tableTriggers:                           tableTriggersCols,
	tableUserPrivileges:                     tableUserPrivilegesCols,
	tableSchemaPrivileges:                   tableSchemaPrivilegesCols,
	tableTablePrivileges:                    tableTablePrivilegesCols,
	tableColumnPrivileges:                   tableColumnPrivilegesCols,
	tableEngines:                            tableEnginesCols,
	tableViews:                              tableViewsCols,
	tableRoutines:                           tableRoutinesCols,
	tableParameters:                         tableParametersCols,
	tableEvents:                             tableEventsCols,
	tableGlobalStatus:                       tableGlobalStatusCols,
	tableGlobalVariables:                    tableGlobalVariablesCols,
	tableSessionStatus:                      tableSessionStatusCols,
	tableOptimizerTrace:                     tableOptimizerTraceCols,
	tableTableSpaces:                        tableTableSpacesCols,
	tableCollationCharacterSetApplicability: tableCollationCharacterSetApplicabilityCols,
	tableProcesslist:                        tableProcesslistCols,
}

func createInfoSchemaTable(handle *Handle, meta *model.TableInfo) *infoschemaTable {
	columns := make([]*table.Column, len(meta.Columns))
	for i, col := range meta.Columns {
		columns[i] = table.ToColumn(col)
	}
	return &infoschemaTable{
		handle: handle,
		meta:   meta,
		cols:   columns,
	}
}

type infoschemaTable struct {
	handle *Handle
	meta   *model.TableInfo
	cols   []*table.Column
	rows   [][]types.Datum
}

// schemasSorter implements the sort.Interface interface, sorts DBInfo by name.
type schemasSorter []*model.DBInfo

func (s schemasSorter) Len() int {
	return len(s)
}

func (s schemasSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s schemasSorter) Less(i, j int) bool {
	return s[i].Name.L < s[j].Name.L
}

func (it *infoschemaTable) getRows(ctx sessionctx.Context, cols []*table.Column) (fullRows [][]types.Datum, err error) {
	is := ctx.GetSessionVars().TxnCtx.InfoSchema.(InfoSchema)
	dbs := is.AllSchemas()
	sort.Sort(schemasSorter(dbs))
	switch it.meta.Name.O {
	case tableSchemata:
		fullRows = dataForSchemata(dbs)
	case tableTables:
		fullRows, err = dataForTables(ctx, dbs)
	case tableColumns:
		fullRows = dataForColumns(ctx, dbs)
	case tableStatistics:
		fullRows = dataForStatistics(dbs)
	case tableCharacterSets:
		fullRows = dataForCharacterSets()
	case tableCollations:
		fullRows = dataForCollations()
	case tableSessionVar:
		fullRows, err = dataForSessionVar(ctx)
	case tableConstraints:
		fullRows = dataForTableConstraints(dbs)
	case tableFiles:
	case tableProfiling:
		if v, ok := ctx.GetSessionVars().GetSystemVar("profiling"); ok && variable.TiDBOptOn(v) {
			fullRows = dataForPseudoProfiling()
		}
	case tablePartitions:
	case tableKeyColumm:
		fullRows = dataForKeyColumnUsage(dbs)
	case tableReferConst:
	case tablePlugins, tableTriggers:
	case tableUserPrivileges:
		fullRows = dataForUserPrivileges(ctx)
	case tableEngines:
		fullRows = dataForEngines()
	case tableViews:
	case tableRoutines:
	// TODO: Fill the following tables.
	case tableSchemaPrivileges:
	case tableTablePrivileges:
	case tableColumnPrivileges:
	case tableParameters:
	case tableEvents:
	case tableGlobalStatus:
	case tableGlobalVariables:
	case tableSessionStatus:
	case tableOptimizerTrace:
	case tableTableSpaces:
	case tableCollationCharacterSetApplicability:
		fullRows = dataForCollationCharacterSetApplicability()
	case tableProcesslist:
		fullRows = dataForProcesslist(ctx)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(cols) == len(it.cols) {
		return
	}
	rows := make([][]types.Datum, len(fullRows))
	for i, fullRow := range fullRows {
		row := make([]types.Datum, len(cols))
		for j, col := range cols {
			row[j] = fullRow[col.Offset]
		}
		rows[i] = row
	}
	return rows, nil
}

func (it *infoschemaTable) IterRecords(ctx sessionctx.Context, startKey kv.Key, cols []*table.Column,
	fn table.RecordIterFunc) error {
	if len(startKey) != 0 {
		return table.ErrUnsupportedOp
	}
	rows, err := it.getRows(ctx, cols)
	if err != nil {
		return errors.Trace(err)
	}
	for i, row := range rows {
		more, err := fn(int64(i), row, cols)
		if err != nil {
			return errors.Trace(err)
		}
		if !more {
			break
		}
	}
	return nil
}

func (it *infoschemaTable) RowWithCols(ctx sessionctx.Context, h int64, cols []*table.Column) ([]types.Datum, error) {
	return nil, table.ErrUnsupportedOp
}

// Row implements table.Table Row interface.
func (it *infoschemaTable) Row(ctx sessionctx.Context, h int64) ([]types.Datum, error) {
	return nil, table.ErrUnsupportedOp
}

func (it *infoschemaTable) Cols() []*table.Column {
	return it.cols
}

func (it *infoschemaTable) WritableCols() []*table.Column {
	return it.cols
}

func (it *infoschemaTable) Indices() []table.Index {
	return nil
}

func (it *infoschemaTable) WritableIndices() []table.Index {
	return nil
}

func (it *infoschemaTable) DeletableIndices() []table.Index {
	return nil
}

func (it *infoschemaTable) RecordPrefix() kv.Key {
	return nil
}

func (it *infoschemaTable) IndexPrefix() kv.Key {
	return nil
}

func (it *infoschemaTable) FirstKey() kv.Key {
	return nil
}

func (it *infoschemaTable) RecordKey(h int64) kv.Key {
	return nil
}

func (it *infoschemaTable) AddRecord(ctx sessionctx.Context, r []types.Datum, skipHandleCheck bool) (recordID int64, err error) {
	return 0, table.ErrUnsupportedOp
}

func (it *infoschemaTable) RemoveRecord(ctx sessionctx.Context, h int64, r []types.Datum) error {
	return table.ErrUnsupportedOp
}

func (it *infoschemaTable) UpdateRecord(ctx sessionctx.Context, h int64, oldData, newData []types.Datum, touched []bool) error {
	return table.ErrUnsupportedOp
}

func (it *infoschemaTable) AllocAutoID(ctx sessionctx.Context) (int64, error) {
	return 0, table.ErrUnsupportedOp
}

func (it *infoschemaTable) Allocator(ctx sessionctx.Context) autoid.Allocator {
	return nil
}

func (it *infoschemaTable) RebaseAutoID(ctx sessionctx.Context, newBase int64, isSetStep bool) error {
	return table.ErrUnsupportedOp
}

func (it *infoschemaTable) Meta() *model.TableInfo {
	return it.meta
}

func (it *infoschemaTable) GetPhysicalID() int64 {
	return it.meta.ID
}

// Seek is the first method called for table scan, we lazy initialize it here.
func (it *infoschemaTable) Seek(ctx sessionctx.Context, h int64) (int64, bool, error) {
	return 0, false, table.ErrUnsupportedOp
}

func (it *infoschemaTable) Type() table.Type {
	return table.VirtualTable
}
