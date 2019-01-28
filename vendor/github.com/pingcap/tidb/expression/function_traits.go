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

package expression

import (
	"github.com/pingcap/parser/ast"
)

// UnCacheableFunctions stores functions which can not be cached to plan cache.
var UnCacheableFunctions = map[string]struct{}{
	ast.Now:              {},
	ast.CurrentTimestamp: {},
	ast.UTCTime:          {},
	ast.Curtime:          {},
	ast.CurrentTime:      {},
	ast.UTCTimestamp:     {},
	ast.UnixTimestamp:    {},
	ast.Sysdate:          {},
	ast.Curdate:          {},
	ast.CurrentDate:      {},
	ast.UTCDate:          {},
	ast.Database:         {},
	ast.CurrentUser:      {},
	ast.User:             {},
	ast.ConnectionID:     {},
	ast.LastInsertId:     {},
	ast.Version:          {},
}

// unFoldableFunctions stores functions which can not be folded duration constant folding stage.
var unFoldableFunctions = map[string]struct{}{
	ast.Sysdate:   {},
	ast.FoundRows: {},
	ast.Rand:      {},
	ast.UUID:      {},
	ast.Sleep:     {},
	ast.RowFunc:   {},
	ast.Values:    {},
	ast.SetVar:    {},
	ast.GetVar:    {},
	ast.GetParam:  {},
}

// inequalFunctions stores functions which cannot be propagated from column equal condition.
var inequalFunctions = map[string]struct{}{
	ast.IsNull: {},
}
