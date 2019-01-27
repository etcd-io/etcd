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

// Metrics for the DDL package.
var (
	JobsGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "tidb",
			Subsystem: "ddl",
			Name:      "waiting_jobs",
			Help:      "Gauge of jobs.",
		}, []string{LblType})

	HandleJobHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tidb",
			Subsystem: "ddl",
			Name:      "handle_job_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of handle jobs",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 20),
		}, []string{LblType, LblResult})

	BatchAddIdxHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tidb",
			Subsystem: "ddl",
			Name:      "batch_add_idx_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of batch handle data",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 20),
		}, []string{LblType})

	SyncerInit            = "init"
	SyncerRestart         = "restart"
	SyncerClear           = "clear"
	SyncerRewatch         = "rewatch"
	DeploySyncerHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tidb",
			Subsystem: "ddl",
			Name:      "deploy_syncer_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of deploy syncer",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 20),
		}, []string{LblType, LblResult})

	UpdateSelfVersionHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tidb",
			Subsystem: "ddl",
			Name:      "update_self_ver_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of update self version",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 20),
		}, []string{LblResult})

	OwnerUpdateGlobalVersion   = "update_global_version"
	OwnerGetGlobalVersion      = "get_global_version"
	OwnerCheckAllVersions      = "check_all_versions"
	OwnerHandleSyncerHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tidb",
			Subsystem: "ddl",
			Name:      "owner_handle_syncer_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of handle syncer",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 20),
		}, []string{LblType, LblResult})

	// Metrics for ddl_worker.go.
	WorkerAddDDLJob         = "add_job"
	WorkerRunDDLJob         = "run_job"
	WorkerFinishDDLJob      = "finish_job"
	WorkerWaitSchemaChanged = "wait_schema_changed"
	DDLWorkerHistogram      = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tidb",
			Subsystem: "ddl",
			Name:      "worker_operation_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of ddl worker operations",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 20),
		}, []string{LblType, LblAction, LblResult})

	CreateDDLInstance = "create_ddl_instance"
	CreateDDL         = "create_ddl"
	IsDDLOwner        = "is_ddl_owner"
	DDLCounter        = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "tidb",
			Subsystem: "ddl",
			Name:      "worker_operation_total",
			Help:      "Counter of creating ddl/worker and isowner.",
		}, []string{LblType})
)

// Label constants.
const (
	LblAction = "action"
)
