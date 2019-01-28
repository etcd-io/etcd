// Copyright 2015 PingCAP, Inc.
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

package model

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/terror"
)

// ActionType is the type for DDL action.
type ActionType byte

// List DDL actions.
const (
	ActionNone               ActionType = 0
	ActionCreateSchema       ActionType = 1
	ActionDropSchema         ActionType = 2
	ActionCreateTable        ActionType = 3
	ActionDropTable          ActionType = 4
	ActionAddColumn          ActionType = 5
	ActionDropColumn         ActionType = 6
	ActionAddIndex           ActionType = 7
	ActionDropIndex          ActionType = 8
	ActionAddForeignKey      ActionType = 9
	ActionDropForeignKey     ActionType = 10
	ActionTruncateTable      ActionType = 11
	ActionModifyColumn       ActionType = 12
	ActionRebaseAutoID       ActionType = 13
	ActionRenameTable        ActionType = 14
	ActionSetDefaultValue    ActionType = 15
	ActionShardRowID         ActionType = 16
	ActionModifyTableComment ActionType = 17
	ActionRenameIndex        ActionType = 18
	ActionAddTablePartition  ActionType = 19
	ActionDropTablePartition ActionType = 20
)

// AddIndexStr is a string related to the operation of "add index".
const AddIndexStr = "add index"

var actionMap = map[ActionType]string{
	ActionCreateSchema:       "create schema",
	ActionDropSchema:         "drop schema",
	ActionCreateTable:        "create table",
	ActionDropTable:          "drop table",
	ActionAddColumn:          "add column",
	ActionDropColumn:         "drop column",
	ActionAddIndex:           AddIndexStr,
	ActionDropIndex:          "drop index",
	ActionAddForeignKey:      "add foreign key",
	ActionDropForeignKey:     "drop foreign key",
	ActionTruncateTable:      "truncate table",
	ActionModifyColumn:       "modify column",
	ActionRebaseAutoID:       "rebase auto_increment ID",
	ActionRenameTable:        "rename table",
	ActionSetDefaultValue:    "set default value",
	ActionShardRowID:         "shard row ID",
	ActionModifyTableComment: "modify table comment",
	ActionRenameIndex:        "rename index",
	ActionAddTablePartition:  "add partition",
	ActionDropTablePartition: "drop table partition",
}

// String return current ddl action in string
func (action ActionType) String() string {
	if v, ok := actionMap[action]; ok {
		return v
	}
	return "none"
}

// HistoryInfo is used for binlog.
type HistoryInfo struct {
	SchemaVersion int64
	DBInfo        *DBInfo
	TableInfo     *TableInfo
	FinishedTS    uint64
}

// AddDBInfo adds schema version and schema information that are used for binlog.
// dbInfo is added in the following operations: create database, drop database.
func (h *HistoryInfo) AddDBInfo(schemaVer int64, dbInfo *DBInfo) {
	h.SchemaVersion = schemaVer
	h.DBInfo = dbInfo
}

// AddTableInfo adds schema version and table information that are used for binlog.
// tblInfo is added except for the following operations: create database, drop database.
func (h *HistoryInfo) AddTableInfo(schemaVer int64, tblInfo *TableInfo) {
	h.SchemaVersion = schemaVer
	h.TableInfo = tblInfo
}

// Clean cleans history information.
func (h *HistoryInfo) Clean() {
	h.SchemaVersion = 0
	h.DBInfo = nil
	h.TableInfo = nil
}

// DDLReorgMeta is meta info of DDL reorganization.
type DDLReorgMeta struct {
	// EndHandle is the last handle of the adding indices table.
	// We should only backfill indices in the range [startHandle, EndHandle].
	EndHandle int64 `json:"end_handle"`
}

// NewDDLReorgMeta new a DDLReorgMeta.
func NewDDLReorgMeta() *DDLReorgMeta {
	return &DDLReorgMeta{
		EndHandle: math.MaxInt64,
	}
}

// Job is for a DDL operation.
type Job struct {
	ID       int64         `json:"id"`
	Type     ActionType    `json:"type"`
	SchemaID int64         `json:"schema_id"`
	TableID  int64         `json:"table_id"`
	State    JobState      `json:"state"`
	Error    *terror.Error `json:"err"`
	// ErrorCount will be increased, every time we meet an error when running job.
	ErrorCount int64 `json:"err_count"`
	// RowCount means the number of rows that are processed.
	RowCount int64         `json:"row_count"`
	Mu       sync.Mutex    `json:"-"`
	Args     []interface{} `json:"-"`
	// RawArgs : We must use json raw message to delay parsing special args.
	RawArgs     json.RawMessage `json:"raw_args"`
	SchemaState SchemaState     `json:"schema_state"`
	// SnapshotVer means snapshot version for this job.
	SnapshotVer uint64 `json:"snapshot_ver"`
	// StartTS uses timestamp allocated by TSO.
	// Now it's the TS when we put the job to TiKV queue.
	StartTS uint64 `json:"start_ts"`
	// DependencyID is the job's ID that the current job depends on.
	DependencyID int64 `json:"dependency_id"`
	// Query string of the ddl job.
	Query      string       `json:"query"`
	BinlogInfo *HistoryInfo `json:"binlog"`

	// Version indicates the DDL job version. For old jobs, it will be 0.
	Version int64 `json:"version"`

	// ReorgMeta is meta info of ddl reorganization.
	// This field is depreciated.
	ReorgMeta *DDLReorgMeta `json:"reorg_meta"`

	// Priority is only used to set the operation priority of adding indices.
	Priority int `json:"priority"`
}

// FinishTableJob is called when a job is finished.
// It updates the job's state information and adds tblInfo to the binlog.
func (job *Job) FinishTableJob(jobState JobState, schemaState SchemaState, ver int64, tblInfo *TableInfo) {
	job.State = jobState
	job.SchemaState = schemaState
	job.BinlogInfo.AddTableInfo(ver, tblInfo)
}

// FinishDBJob is called when a job is finished.
// It updates the job's state information and adds dbInfo the binlog.
func (job *Job) FinishDBJob(jobState JobState, schemaState SchemaState, ver int64, dbInfo *DBInfo) {
	job.State = jobState
	job.SchemaState = schemaState
	job.BinlogInfo.AddDBInfo(ver, dbInfo)
}

// TSConvert2Time converts timestamp to time.
func TSConvert2Time(ts uint64) time.Time {
	t := int64(ts >> 18) // 18 is for the logical time.
	return time.Unix(t/1e3, (t%1e3)*1e6)
}

// SetRowCount sets the number of rows. Make sure it can pass `make race`.
func (job *Job) SetRowCount(count int64) {
	job.Mu.Lock()
	defer job.Mu.Unlock()

	job.RowCount = count
}

// GetRowCount gets the number of rows. Make sure it can pass `make race`.
func (job *Job) GetRowCount() int64 {
	job.Mu.Lock()
	defer job.Mu.Unlock()

	return job.RowCount
}

// Encode encodes job with json format.
// updateRawArgs is used to determine whether to update the raw args.
func (job *Job) Encode(updateRawArgs bool) ([]byte, error) {
	var err error
	if updateRawArgs {
		job.RawArgs, err = json.Marshal(job.Args)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	var b []byte
	job.Mu.Lock()
	defer job.Mu.Unlock()
	b, err = json.Marshal(job)

	return b, errors.Trace(err)
}

// Decode decodes job from the json buffer, we must use DecodeArgs later to
// decode special args for this job.
func (job *Job) Decode(b []byte) error {
	err := json.Unmarshal(b, job)
	return errors.Trace(err)
}

// DecodeArgs decodes job args.
func (job *Job) DecodeArgs(args ...interface{}) error {
	job.Args = args
	err := json.Unmarshal(job.RawArgs, &job.Args)
	return errors.Trace(err)
}

// String implements fmt.Stringer interface.
func (job *Job) String() string {
	rowCount := job.GetRowCount()
	return fmt.Sprintf("ID:%d, Type:%s, State:%s, SchemaState:%s, SchemaID:%d, TableID:%d, RowCount:%d, ArgLen:%d, start time: %v, Err:%v, ErrCount:%d, SnapshotVersion:%v",
		job.ID, job.Type, job.State, job.SchemaState, job.SchemaID, job.TableID, rowCount, len(job.Args), TSConvert2Time(job.StartTS), job.Error, job.ErrorCount, job.SnapshotVer)
}

func (job *Job) hasDependentSchema(other *Job) (bool, error) {
	if other.Type == ActionDropSchema || other.Type == ActionCreateSchema {
		if other.SchemaID == job.SchemaID {
			return true, nil
		}
		if job.Type == ActionRenameTable {
			var oldSchemaID int64
			if err := job.DecodeArgs(&oldSchemaID); err != nil {
				return false, errors.Trace(err)
			}
			if other.SchemaID == oldSchemaID {
				return true, nil
			}
		}
	}
	return false, nil
}

// IsDependentOn returns whether the job depends on "other".
// How to check the job depends on "other"?
// 1. The two jobs handle the same database when one of the two jobs is an ActionDropSchema or ActionCreateSchema type.
// 2. Or the two jobs handle the same table.
func (job *Job) IsDependentOn(other *Job) (bool, error) {
	isDependent, err := job.hasDependentSchema(other)
	if err != nil || isDependent {
		return isDependent, errors.Trace(err)
	}
	isDependent, err = other.hasDependentSchema(job)
	if err != nil || isDependent {
		return isDependent, errors.Trace(err)
	}

	// TODO: If a job is ActionRenameTable, we need to check table name.
	if other.TableID == job.TableID {
		return true, nil
	}
	return false, nil
}

// IsFinished returns whether job is finished or not.
// If the job state is Done or Cancelled, it is finished.
func (job *Job) IsFinished() bool {
	return job.State == JobStateDone || job.State == JobStateRollbackDone || job.State == JobStateCancelled
}

// IsCancelled returns whether the job is cancelled or not.
func (job *Job) IsCancelled() bool {
	return job.State == JobStateCancelled
}

// IsRollbackDone returns whether the job is rolled back or not.
func (job *Job) IsRollbackDone() bool {
	return job.State == JobStateRollbackDone
}

// IsRollingback returns whether the job is rolling back or not.
func (job *Job) IsRollingback() bool {
	return job.State == JobStateRollingback
}

// IsCancelling returns whether the job is cancelling or not.
func (job *Job) IsCancelling() bool {
	return job.State == JobStateCancelling
}

// IsSynced returns whether the DDL modification is synced among all TiDB servers.
func (job *Job) IsSynced() bool {
	return job.State == JobStateSynced
}

// IsDone returns whether job is done.
func (job *Job) IsDone() bool {
	return job.State == JobStateDone
}

// IsRunning returns whether job is still running or not.
func (job *Job) IsRunning() bool {
	return job.State == JobStateRunning
}

// JobState is for job state.
type JobState byte

// List job states.
const (
	JobStateNone    JobState = 0
	JobStateRunning JobState = 1
	// When DDL encountered an unrecoverable error at reorganization state,
	// some keys has been added already, we need to remove them.
	// JobStateRollingback is the state to do the rolling back job.
	JobStateRollingback  JobState = 2
	JobStateRollbackDone JobState = 3
	JobStateDone         JobState = 4
	JobStateCancelled    JobState = 5
	// JobStateSynced is used to mark the information about the completion of this job
	// has been synchronized to all servers.
	JobStateSynced JobState = 6
	// JobStateCancelling is used to mark the DDL job is cancelled by the client, but the DDL work hasn't handle it.
	JobStateCancelling JobState = 7
)

// String implements fmt.Stringer interface.
func (s JobState) String() string {
	switch s {
	case JobStateRunning:
		return "running"
	case JobStateRollingback:
		return "rollingback"
	case JobStateRollbackDone:
		return "rollback done"
	case JobStateDone:
		return "done"
	case JobStateCancelled:
		return "cancelled"
	case JobStateCancelling:
		return "cancelling"
	case JobStateSynced:
		return "synced"
	default:
		return "none"
	}
}

// SchemaDiff contains the schema modification at a particular schema version.
// It is used to reduce schema reload cost.
type SchemaDiff struct {
	Version  int64      `json:"version"`
	Type     ActionType `json:"type"`
	SchemaID int64      `json:"schema_id"`
	TableID  int64      `json:"table_id"`

	// OldTableID is the table ID before truncate, only used by truncate table DDL.
	OldTableID int64 `json:"old_table_id"`
	// OldSchemaID is the schema ID before rename table, only used by rename table DDL.
	OldSchemaID int64 `json:"old_schema_id"`
}
