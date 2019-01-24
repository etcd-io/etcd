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

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics
var (
	GlobalAutoID      = "global"
	TableAutoIDAlloc  = "alloc"
	TableAutoIDRebase = "rebase"
	AutoIDHistogram   = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tidb",
			Subsystem: "autoid",
			Name:      "operation_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of handled autoid.",
			Buckets:   prometheus.ExponentialBuckets(0.0005, 2, 22),
		}, []string{LblType, LblResult})

	GetSchemaDiff    = "get_schema_diff"
	SetSchemaDiff    = "set_schema_diff"
	GetDDLJobByIdx   = "get_ddl_job"
	UpdateDDLJob     = "update_ddl_job"
	GetHistoryDDLJob = "get_history_ddl_job"

	MetaHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "tidb",
			Subsystem: "meta",
			Name:      "operation_duration_seconds",
			Help:      "Bucketed histogram of processing time (s) of tidb meta data operations.",
			Buckets:   prometheus.ExponentialBuckets(0.0005, 2, 22),
		}, []string{LblType, LblResult})
)
