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

package variable

import (
	"os"

	"github.com/pingcap/parser/mysql"
)

/*
	Steps to add a new TiDB specific system variable:

	1. Add a new variable name with comment in this file.
	2. Add the default value of the new variable in this file.
	3. Add SysVar instance in 'defaultSysVars' slice with the default value.
	4. Add a field in `SessionVars`.
	5. Update the `NewSessionVars` function to set the field to its default value.
	6. Update the `variable.SetSessionSystemVar` function to use the new value when SET statement is executed.
	7. If it is a global variable, add it in `session.loadCommonGlobalVarsSQL`.
	8. Use this variable to control the behavior in code.
*/

// TiDB system variable names that only in session scope.
const (
	// tidb_snapshot is used for reading history data, the default value is empty string.
	// The value can be a datetime string like '2017-11-11 20:20:20' or a tso string. When this variable is set, the session reads history data of that time.
	TiDBSnapshot = "tidb_snapshot"

	// tidb_opt_agg_push_down is used to enable/disable the optimizer rule of aggregation push down.
	TiDBOptAggPushDown = "tidb_opt_agg_push_down"

	// tidb_opt_write_row_id is used to enable/disable the operations of insert、replace and update to _tidb_rowid.
	TiDBOptWriteRowID = "tidb_opt_write_row_id"

	// Auto analyze will run if (table modify count)/(table row count) is greater than this value.
	TiDBAutoAnalyzeRatio = "tidb_auto_analyze_ratio"

	// Auto analyze will run if current time is within start time and end time.
	TiDBAutoAnalyzeStartTime = "tidb_auto_analyze_start_time"
	TiDBAutoAnalyzeEndTime   = "tidb_auto_analyze_end_time"

	// tidb_checksum_table_concurrency is used to speed up the ADMIN CHECKSUM TABLE
	// statement, when a table has multiple indices, those indices can be
	// scanned concurrently, with the cost of higher system performance impact.
	TiDBChecksumTableConcurrency = "tidb_checksum_table_concurrency"

	// TiDBCurrentTS is used to get the current transaction timestamp.
	// It is read-only.
	TiDBCurrentTS = "tidb_current_ts"

	// tidb_config is a read-only variable that shows the config of the current server.
	TiDBConfig = "tidb_config"

	// tidb_batch_insert is used to enable/disable auto-split insert data. If set this option on, insert executor will automatically
	// insert data into multiple batches and use a single txn for each batch. This will be helpful when inserting large data.
	TiDBBatchInsert = "tidb_batch_insert"

	// tidb_batch_delete is used to enable/disable auto-split delete data. If set this option on, delete executor will automatically
	// split data into multiple batches and use a single txn for each batch. This will be helpful when deleting large data.
	TiDBBatchDelete = "tidb_batch_delete"

	// tidb_dml_batch_size is used to split the insert/delete data into small batches.
	// It only takes effort when tidb_batch_insert/tidb_batch_delete is on.
	// Its default value is 20000. When the row size is large, 20k rows could be larger than 100MB.
	// User could change it to a smaller one to avoid breaking the transaction size limitation.
	TiDBDMLBatchSize = "tidb_dml_batch_size"

	// The following session variables controls the memory quota during query execution.
	// "tidb_mem_quota_query":				control the memory quota of a query.
	// "tidb_mem_quota_hashjoin": 			control the memory quota of "HashJoinExec".
	// "tidb_mem_quota_mergejoin": 			control the memory quota of "MergeJoinExec".
	// "tidb_mem_quota_sort":     			control the memory quota of "SortExec".
	// "tidb_mem_quota_topn":     			control the memory quota of "TopNExec".
	// "tidb_mem_quota_indexlookupreader":	control the memory quota of "IndexLookUpExecutor".
	// "tidb_mem_quota_indexlookupjoin":	control the memory quota of "IndexLookUpJoin".
	// "tidb_mem_quota_nestedloopapply": 	control the memory quota of "NestedLoopApplyExec".
	TIDBMemQuotaQuery             = "tidb_mem_quota_query"             // Bytes.
	TIDBMemQuotaHashJoin          = "tidb_mem_quota_hashjoin"          // Bytes.
	TIDBMemQuotaMergeJoin         = "tidb_mem_quota_mergejoin"         // Bytes.
	TIDBMemQuotaSort              = "tidb_mem_quota_sort"              // Bytes.
	TIDBMemQuotaTopn              = "tidb_mem_quota_topn"              // Bytes.
	TIDBMemQuotaIndexLookupReader = "tidb_mem_quota_indexlookupreader" // Bytes.
	TIDBMemQuotaIndexLookupJoin   = "tidb_mem_quota_indexlookupjoin"   // Bytes.
	TIDBMemQuotaNestedLoopApply   = "tidb_mem_quota_nestedloopapply"   // Bytes.

	// tidb_general_log is used to log every query in the server in info level.
	TiDBGeneralLog = "tidb_general_log"

	// tidb_slow_log_threshold is used to set the slow log threshold in the server.
	TiDBSlowLogThreshold = "tidb_slow_log_threshold"

	// tidb_query_log_max_len is used to set the max length of the query in the log.
	TiDBQueryLogMaxLen = "tidb_query_log_max_len"

	// tidb_retry_limit is the maximum number of retries when committing a transaction.
	TiDBRetryLimit = "tidb_retry_limit"

	// tidb_disable_txn_auto_retry disables transaction auto retry.
	TiDBDisableTxnAutoRetry = "tidb_disable_txn_auto_retry"

	// tidb_enable_streaming enables TiDB to use streaming API for coprocessor requests.
	TiDBEnableStreaming = "tidb_enable_streaming"

	// tidb_optimizer_selectivity_level is used to control the selectivity estimation level.
	TiDBOptimizerSelectivityLevel = "tidb_optimizer_selectivity_level"

	// tidb_enable_table_partition is used to enable table partition feature.
	TiDBEnableTablePartition = "tidb_enable_table_partition"
)

// TiDB system variable names that both in session and global scope.
const (
	// tidb_build_stats_concurrency is used to speed up the ANALYZE statement, when a table has multiple indices,
	// those indices can be scanned concurrently, with the cost of higher system performance impact.
	TiDBBuildStatsConcurrency = "tidb_build_stats_concurrency"

	// tidb_distsql_scan_concurrency is used to set the concurrency of a distsql scan task.
	// A distsql scan task can be a table scan or a index scan, which may be distributed to many TiKV nodes.
	// Higher concurrency may reduce latency, but with the cost of higher memory usage and system performance impact.
	// If the query has a LIMIT clause, high concurrency makes the system do much more work than needed.
	TiDBDistSQLScanConcurrency = "tidb_distsql_scan_concurrency"

	// tidb_opt_insubquery_unfold is used to enable/disable the optimizer rule of in subquery unfold.
	TiDBOptInSubqUnFolding = "tidb_opt_insubquery_unfold"

	// tidb_index_join_batch_size is used to set the batch size of a index lookup join.
	// The index lookup join fetches batches of data from outer executor and constructs ranges for inner executor.
	// This value controls how much of data in a batch to do the index join.
	// Large value may reduce the latency but consumes more system resource.
	TiDBIndexJoinBatchSize = "tidb_index_join_batch_size"

	// tidb_index_lookup_size is used for index lookup executor.
	// The index lookup executor first scan a batch of handles from a index, then use those handles to lookup the table
	// rows, this value controls how much of handles in a batch to do a lookup task.
	// Small value sends more RPCs to TiKV, consume more system resource.
	// Large value may do more work than needed if the query has a limit.
	TiDBIndexLookupSize = "tidb_index_lookup_size"

	// tidb_index_lookup_concurrency is used for index lookup executor.
	// A lookup task may have 'tidb_index_lookup_size' of handles at maximun, the handles may be distributed
	// in many TiKV nodes, we executes multiple concurrent index lookup tasks concurrently to reduce the time
	// waiting for a task to finish.
	// Set this value higher may reduce the latency but consumes more system resource.
	TiDBIndexLookupConcurrency = "tidb_index_lookup_concurrency"

	// tidb_index_lookup_join_concurrency is used for index lookup join executor.
	// IndexLookUpJoin starts "tidb_index_lookup_join_concurrency" inner workers
	// to fetch inner rows and join the matched (outer, inner) row pairs.
	TiDBIndexLookupJoinConcurrency = "tidb_index_lookup_join_concurrency"

	// tidb_index_serial_scan_concurrency is used for controlling the concurrency of index scan operation
	// when we need to keep the data output order the same as the order of index data.
	TiDBIndexSerialScanConcurrency = "tidb_index_serial_scan_concurrency"

	// tidb_max_chunk_capacity is used to control the max chunk size during query execution.
	TiDBMaxChunkSize = "tidb_max_chunk_size"

	// tidb_skip_utf8_check skips the UTF8 validate process, validate UTF8 has performance cost, if we can make sure
	// the input string values are valid, we can skip the check.
	TiDBSkipUTF8Check = "tidb_skip_utf8_check"

	// tidb_hash_join_concurrency is used for hash join executor.
	// The hash join outer executor starts multiple concurrent join workers to probe the hash table.
	TiDBHashJoinConcurrency = "tidb_hash_join_concurrency"

	// tidb_projection_concurrency is used for projection operator.
	// This variable controls the worker number of projection operator.
	TiDBProjectionConcurrency = "tidb_projection_concurrency"

	// tidb_hashagg_partial_concurrency is used for hash agg executor.
	// The hash agg executor starts multiple concurrent partial workers to do partial aggregate works.
	TiDBHashAggPartialConcurrency = "tidb_hashagg_partial_concurrency"

	// tidb_hashagg_final_concurrency is used for hash agg executor.
	// The hash agg executor starts multiple concurrent final workers to do final aggregate works.
	TiDBHashAggFinalConcurrency = "tidb_hashagg_final_concurrency"

	// tidb_backoff_lock_fast is used for tikv backoff base time in milliseconds.
	TiDBBackoffLockFast = "tidb_backoff_lock_fast"

	// tidb_ddl_reorg_worker_cnt defines the count of ddl reorg workers.
	TiDBDDLReorgWorkerCount = "tidb_ddl_reorg_worker_cnt"

	// tidb_ddl_reorg_batch_size defines the transaction batch size of ddl reorg workers.
	TiDBDDLReorgBatchSize = "tidb_ddl_reorg_batch_size"

	// tidb_ddl_reorg_priority defines the operations priority of adding indices.
	// It can be: PRIORITY_LOW, PRIORITY_NORMAL, PRIORITY_HIGH
	TiDBDDLReorgPriority = "tidb_ddl_reorg_priority"

	// tidb_force_priority defines the operations priority of all statements.
	// It can be "NO_PRIORITY", "LOW_PRIORITY", "HIGH_PRIORITY", "DELAYED"
	TiDBForcePriority = "tidb_force_priority"
)

// Default TiDB system variable values.
const (
	DefHostname                      = "localhost"
	DefIndexLookupConcurrency        = 4
	DefIndexLookupJoinConcurrency    = 4
	DefIndexSerialScanConcurrency    = 1
	DefIndexJoinBatchSize            = 25000
	DefIndexLookupSize               = 20000
	DefDistSQLScanConcurrency        = 15
	DefBuildStatsConcurrency         = 4
	DefAutoAnalyzeRatio              = 0.5
	DefAutoAnalyzeStartTime          = "00:00 +0000"
	DefAutoAnalyzeEndTime            = "23:59 +0000"
	DefChecksumTableConcurrency      = 4
	DefSkipUTF8Check                 = false
	DefOptAggPushDown                = false
	DefOptInSubqUnfolding            = false
	DefOptWriteRowID                 = false
	DefBatchInsert                   = false
	DefBatchDelete                   = false
	DefCurretTS                      = 0
	DefMaxChunkSize                  = 32
	DefDMLBatchSize                  = 20000
	DefTiDBMemQuotaHashJoin          = 32 << 30 // 32GB.
	DefTiDBMemQuotaMergeJoin         = 32 << 30 // 32GB.
	DefTiDBMemQuotaSort              = 32 << 30 // 32GB.
	DefTiDBMemQuotaTopn              = 32 << 30 // 32GB.
	DefTiDBMemQuotaIndexLookupReader = 32 << 30 // 32GB.
	DefTiDBMemQuotaIndexLookupJoin   = 32 << 30 // 32GB.
	DefTiDBMemQuotaNestedLoopApply   = 32 << 30 // 32GB.
	DefTiDBGeneralLog                = 0
	DefTiDBRetryLimit                = 10
	DefTiDBDisableTxnAutoRetry       = false
	DefTiDBHashJoinConcurrency       = 5
	DefTiDBProjectionConcurrency     = 4
	DefTiDBOptimizerSelectivityLevel = 0
	DefTiDBDDLReorgWorkerCount       = 16
	DefTiDBDDLReorgBatchSize         = 1024
	DefTiDBHashAggPartialConcurrency = 4
	DefTiDBHashAggFinalConcurrency   = 4
	DefTiDBForcePriority             = mysql.NoPriority
)

// Process global variables.
var (
	ProcessGeneralLog      uint32
	ddlReorgWorkerCounter  int32 = DefTiDBDDLReorgWorkerCount
	maxDDLReorgWorkerCount int32 = 128
	ddlReorgBatchSize      int32 = DefTiDBDDLReorgBatchSize
	// Export for testing.
	MaxDDLReorgBatchSize int32  = 10240
	MinDDLReorgBatchSize int32  = 32
	DDLSlowOprThreshold  uint32 = 300 // DDLSlowOprThreshold is the threshold for ddl slow operations, uint is millisecond.
	ForcePriority               = int32(DefTiDBForcePriority)
	ServerHostname, _           = os.Hostname()
)
