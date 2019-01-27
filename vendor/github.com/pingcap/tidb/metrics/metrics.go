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

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// PanicCounter measures the count of panics.
	PanicCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tidb",
			Subsystem: "server",
			Name:      "panic_total",
			Help:      "Counter of panic.",
		}, []string{LblType})
)

// metrics labels.
const (
	LabelSession  = "session"
	LabelDomain   = "domain"
	LabelDDLOwner = "ddl-owner"
	LabelDDL      = "ddl"
	LabelGCWorker = "gcworker"
	LabelAnalyze  = "analyze"

	opSucc   = "ok"
	opFailed = "err"
)

// RetLabel returns "ok" when err == nil and "err" when err != nil.
// This could be useful when you need to observe the operation result.
func RetLabel(err error) string {
	if err == nil {
		return opSucc
	}
	return opFailed
}

// RegisterMetrics registers the metrics which are ONLY used in TiDB server.
func RegisterMetrics() {
	prometheus.MustRegister(AutoAnalyzeCounter)
	prometheus.MustRegister(AutoAnalyzeHistogram)
	prometheus.MustRegister(AutoIDHistogram)
	prometheus.MustRegister(BatchAddIdxHistogram)
	prometheus.MustRegister(CampaignOwnerCounter)
	prometheus.MustRegister(ConnGauge)
	prometheus.MustRegister(CriticalErrorCounter)
	prometheus.MustRegister(DDLCounter)
	prometheus.MustRegister(DDLWorkerHistogram)
	prometheus.MustRegister(DeploySyncerHistogram)
	prometheus.MustRegister(DistSQLPartialCountHistogram)
	prometheus.MustRegister(DistSQLQueryHistgram)
	prometheus.MustRegister(DistSQLScanKeysHistogram)
	prometheus.MustRegister(DistSQLScanKeysPartialHistogram)
	prometheus.MustRegister(DumpFeedbackCounter)
	prometheus.MustRegister(ExecuteErrorCounter)
	prometheus.MustRegister(ExecutorCounter)
	prometheus.MustRegister(GetTokenDurationHistogram)
	prometheus.MustRegister(HandShakeErrorCounter)
	prometheus.MustRegister(HandleJobHistogram)
	prometheus.MustRegister(JobsGauge)
	prometheus.MustRegister(KeepAliveCounter)
	prometheus.MustRegister(LoadPrivilegeCounter)
	prometheus.MustRegister(LoadSchemaCounter)
	prometheus.MustRegister(LoadSchemaDuration)
	prometheus.MustRegister(MetaHistogram)
	prometheus.MustRegister(NewSessionHistogram)
	prometheus.MustRegister(OwnerHandleSyncerHistogram)
	prometheus.MustRegister(PanicCounter)
	prometheus.MustRegister(PlanCacheCounter)
	prometheus.MustRegister(PseudoEstimation)
	prometheus.MustRegister(QueryDurationHistogram)
	prometheus.MustRegister(QueryTotalCounter)
	prometheus.MustRegister(SchemaLeaseErrorCounter)
	prometheus.MustRegister(ServerEventCounter)
	prometheus.MustRegister(SessionExecuteCompileDuration)
	prometheus.MustRegister(SessionExecuteParseDuration)
	prometheus.MustRegister(SessionExecuteRunDuration)
	prometheus.MustRegister(SessionRestrictedSQLCounter)
	prometheus.MustRegister(SessionRetry)
	prometheus.MustRegister(SessionRetryErrorCounter)
	prometheus.MustRegister(StatementPerTransaction)
	prometheus.MustRegister(StatsInaccuracyRate)
	prometheus.MustRegister(StmtNodeCounter)
	prometheus.MustRegister(StoreQueryFeedbackCounter)
	prometheus.MustRegister(TiKVBackoffCounter)
	prometheus.MustRegister(TiKVBackoffHistogram)
	prometheus.MustRegister(TiKVConnPoolHistogram)
	prometheus.MustRegister(TiKVCoprocessorHistogram)
	prometheus.MustRegister(TiKVLoadSafepointCounter)
	prometheus.MustRegister(TiKVLockResolverCounter)
	prometheus.MustRegister(TiKVRawkvCmdHistogram)
	prometheus.MustRegister(TiKVRawkvSizeHistogram)
	prometheus.MustRegister(TiKVRegionCacheCounter)
	prometheus.MustRegister(TiKVRegionErrorCounter)
	prometheus.MustRegister(TiKVSecondaryLockCleanupFailureCounter)
	prometheus.MustRegister(TiKVSendReqHistogram)
	prometheus.MustRegister(TiKVSnapshotCounter)
	prometheus.MustRegister(TiKVTxnCmdHistogram)
	prometheus.MustRegister(TiKVTxnCounter)
	prometheus.MustRegister(TiKVTxnRegionsNumHistogram)
	prometheus.MustRegister(TiKVTxnWriteKVCountHistogram)
	prometheus.MustRegister(TiKVTxnWriteSizeHistogram)
	prometheus.MustRegister(TimeJumpBackCounter)
	prometheus.MustRegister(TransactionCounter)
	prometheus.MustRegister(TransactionDuration)
	prometheus.MustRegister(UpdateSelfVersionHistogram)
	prometheus.MustRegister(UpdateStatsCounter)
	prometheus.MustRegister(WatchOwnerCounter)
	prometheus.MustRegister(GCActionRegionResultCounter)
	prometheus.MustRegister(GCConfigGauge)
	prometheus.MustRegister(GCHistogram)
	prometheus.MustRegister(GCJobFailureCounter)
	prometheus.MustRegister(GCRegionTooManyLocksCounter)
	prometheus.MustRegister(GCWorkerCounter)
	prometheus.MustRegister(TSFutureWaitDuration)
	prometheus.MustRegister(TotalQueryProcHistogram)
	prometheus.MustRegister(TotalCopProcHistogram)
	prometheus.MustRegister(TotalCopWaitHistogram)
}
