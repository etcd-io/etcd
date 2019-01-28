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

package perfschema

import (
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/table"
)

// perfSchema is used for tables in performance_schema.
type perfSchema struct {
	dbInfo  *model.DBInfo
	tables  map[string]*model.TableInfo
	mTables map[string]table.Table // Memory tables for perfSchema
}

var handle = newPerfHandle()

// newPerfHandle creates a new perfSchema on store.
func newPerfHandle() *perfSchema {
	schema := &perfSchema{}
	schema.initialize()
	return schema
}

// GetDBMeta returns db info for PerformanceSchema.
func GetDBMeta() *model.DBInfo {
	return handle.GetDBMeta()
}

// GetTable returns table instance for name.
func GetTable(name string) (table.Table, bool) {
	return handle.GetTable(name)
}
