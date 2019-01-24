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

package variable

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/tidb/config"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/timeutil"
)

// secondsPerYear represents seconds in a normal year. Leap year is not considered here.
const secondsPerYear = 60 * 60 * 24 * 365

// SetDDLReorgWorkerCounter sets ddlReorgWorkerCounter count.
// Max worker count is maxDDLReorgWorkerCount.
func SetDDLReorgWorkerCounter(cnt int32) {
	if cnt > maxDDLReorgWorkerCount {
		cnt = maxDDLReorgWorkerCount
	}
	atomic.StoreInt32(&ddlReorgWorkerCounter, cnt)
}

// GetDDLReorgWorkerCounter gets ddlReorgWorkerCounter.
func GetDDLReorgWorkerCounter() int32 {
	return atomic.LoadInt32(&ddlReorgWorkerCounter)
}

// GetSessionSystemVar gets a system variable.
// If it is a session only variable, use the default value defined in code.
// Returns error if there is no such variable.
func GetSessionSystemVar(s *SessionVars, key string) (string, error) {
	key = strings.ToLower(key)
	gVal, ok, err := GetSessionOnlySysVars(s, key)
	if err != nil || ok {
		return gVal, errors.Trace(err)
	}
	gVal, err = s.GlobalVarsAccessor.GetGlobalSysVar(key)
	if err != nil {
		return "", errors.Trace(err)
	}
	s.systems[key] = gVal
	return gVal, nil
}

// GetSessionOnlySysVars get the default value defined in code for session only variable.
// The return bool value indicates whether it's a session only variable.
func GetSessionOnlySysVars(s *SessionVars, key string) (string, bool, error) {
	sysVar := SysVars[key]
	if sysVar == nil {
		return "", false, UnknownSystemVar.GenWithStackByArgs(key)
	}
	// For virtual system variables:
	switch sysVar.Name {
	case TiDBCurrentTS:
		return fmt.Sprintf("%d", s.TxnCtx.StartTS), true, nil
	case TiDBGeneralLog:
		return fmt.Sprintf("%d", atomic.LoadUint32(&ProcessGeneralLog)), true, nil
	case TiDBConfig:
		conf := config.GetGlobalConfig()
		j, err := json.MarshalIndent(conf, "", "\t")
		if err != nil {
			return "", false, errors.Trace(err)
		}
		return string(j), true, nil
	case TiDBForcePriority:
		return mysql.Priority2Str[mysql.PriorityEnum(atomic.LoadInt32(&ForcePriority))], true, nil
	case TiDBSlowLogThreshold:
		return strconv.FormatUint(atomic.LoadUint64(&config.GetGlobalConfig().Log.SlowThreshold), 10), true, nil
	case TiDBQueryLogMaxLen:
		return strconv.FormatUint(atomic.LoadUint64(&config.GetGlobalConfig().Log.QueryLogMaxLen), 10), true, nil
	}
	sVal, ok := s.systems[key]
	if ok {
		return sVal, true, nil
	}
	if sysVar.Scope&ScopeGlobal == 0 {
		// None-Global variable can use pre-defined default value.
		return sysVar.Value, true, nil
	}
	return "", false, nil
}

// GetGlobalSystemVar gets a global system variable.
func GetGlobalSystemVar(s *SessionVars, key string) (string, error) {
	key = strings.ToLower(key)
	gVal, ok, err := GetScopeNoneSystemVar(key)
	if err != nil || ok {
		return gVal, errors.Trace(err)
	}
	gVal, err = s.GlobalVarsAccessor.GetGlobalSysVar(key)
	if err != nil {
		return "", errors.Trace(err)
	}
	return gVal, nil
}

// GetScopeNoneSystemVar checks the validation of `key`,
// and return the default value if its scope is `ScopeNone`.
func GetScopeNoneSystemVar(key string) (string, bool, error) {
	sysVar := SysVars[key]
	if sysVar == nil {
		return "", false, UnknownSystemVar.GenWithStackByArgs(key)
	}
	if sysVar.Scope == ScopeNone {
		return sysVar.Value, true, nil
	}
	return "", false, nil
}

// epochShiftBits is used to reserve logical part of the timestamp.
const epochShiftBits = 18

// SetSessionSystemVar sets system variable and updates SessionVars states.
func SetSessionSystemVar(vars *SessionVars, name string, value types.Datum) error {
	name = strings.ToLower(name)
	sysVar := SysVars[name]
	if sysVar == nil {
		return UnknownSystemVar
	}
	sVal := ""
	var err error
	if !value.IsNull() {
		sVal, err = value.ToString()
	}
	if err != nil {
		return errors.Trace(err)
	}
	sVal, err = ValidateSetSystemVar(vars, name, sVal)
	if err != nil {
		return errors.Trace(err)
	}
	return vars.SetSystemVar(name, sVal)
}

// ValidateGetSystemVar checks if system variable exists and validates its scope when get system variable.
func ValidateGetSystemVar(name string, isGlobal bool) error {
	sysVar, exists := SysVars[name]
	if !exists {
		return UnknownSystemVar.GenWithStackByArgs(name)
	}
	switch sysVar.Scope {
	case ScopeGlobal, ScopeNone:
		if !isGlobal {
			return ErrIncorrectScope.GenWithStackByArgs(name, "GLOBAL")
		}
	case ScopeSession:
		if isGlobal {
			return ErrIncorrectScope.GenWithStackByArgs(name, "SESSION")
		}
	}
	return nil
}

func checkUInt64SystemVar(name, value string, min, max uint64, vars *SessionVars) (string, error) {
	if value[0] == '-' {
		_, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return value, ErrWrongTypeForVar.GenWithStackByArgs(name)
		}
		vars.StmtCtx.AppendWarning(ErrTruncatedWrongValue.GenWithStackByArgs(name, value))
		return fmt.Sprintf("%d", min), nil
	}
	val, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return value, ErrWrongTypeForVar.GenWithStackByArgs(name)
	}
	if val < min {
		vars.StmtCtx.AppendWarning(ErrTruncatedWrongValue.GenWithStackByArgs(name, value))
		return fmt.Sprintf("%d", min), nil
	}
	if val > max {
		vars.StmtCtx.AppendWarning(ErrTruncatedWrongValue.GenWithStackByArgs(name, value))
		return fmt.Sprintf("%d", max), nil
	}
	return value, nil
}

func checkInt64SystemVar(name, value string, min, max int64, vars *SessionVars) (string, error) {
	val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return value, ErrWrongTypeForVar.GenWithStackByArgs(name)
	}
	if val < min {
		vars.StmtCtx.AppendWarning(ErrTruncatedWrongValue.GenWithStackByArgs(name, value))
		return fmt.Sprintf("%d", min), nil
	}
	if val > max {
		vars.StmtCtx.AppendWarning(ErrTruncatedWrongValue.GenWithStackByArgs(name, value))
		return fmt.Sprintf("%d", max), nil
	}
	return value, nil
}

// ValidateSetSystemVar checks if system variable satisfies specific restriction.
func ValidateSetSystemVar(vars *SessionVars, name string, value string) (string, error) {
	if strings.EqualFold(value, "DEFAULT") {
		if val := GetSysVar(name); val != nil {
			return val.Value, nil
		}
		return value, UnknownSystemVar.GenWithStackByArgs(name)
	}
	switch name {
	case ConnectTimeout:
		return checkUInt64SystemVar(name, value, 2, secondsPerYear, vars)
	case DefaultWeekFormat:
		return checkUInt64SystemVar(name, value, 0, 7, vars)
	case DelayKeyWrite:
		if strings.EqualFold(value, "ON") || value == "1" {
			return "ON", nil
		} else if strings.EqualFold(value, "OFF") || value == "0" {
			return "OFF", nil
		} else if strings.EqualFold(value, "ALL") || value == "2" {
			return "ALL", nil
		}
		return value, ErrWrongValueForVar.GenWithStackByArgs(name, value)
	case FlushTime:
		return checkUInt64SystemVar(name, value, 0, secondsPerYear, vars)
	case GroupConcatMaxLen:
		// The reasonable range of 'group_concat_max_len' is 4~18446744073709551615(64-bit platforms)
		// See https://dev.mysql.com/doc/refman/8.0/en/server-system-variables.html#sysvar_group_concat_max_len for details
		return checkUInt64SystemVar(name, value, 4, math.MaxUint64, vars)
	case InteractiveTimeout:
		return checkUInt64SystemVar(name, value, 1, secondsPerYear, vars)
	case InnodbCommitConcurrency:
		return checkUInt64SystemVar(name, value, 0, 1000, vars)
	case InnodbFastShutdown:
		return checkUInt64SystemVar(name, value, 0, 2, vars)
	case InnodbLockWaitTimeout:
		return checkUInt64SystemVar(name, value, 1, 1073741824, vars)
	// See "https://dev.mysql.com/doc/refman/5.7/en/server-system-variables.html#sysvar_max_allowed_packet"
	case MaxAllowedPacket:
		return checkUInt64SystemVar(name, value, 1024, 1073741824, vars)
	case MaxConnections:
		return checkUInt64SystemVar(name, value, 1, 100000, vars)
	case MaxConnectErrors:
		return checkUInt64SystemVar(name, value, 1, math.MaxUint64, vars)
	case MaxSortLength:
		return checkUInt64SystemVar(name, value, 4, 8388608, vars)
	case MaxSpRecursionDepth:
		return checkUInt64SystemVar(name, value, 0, 255, vars)
	case MaxUserConnections:
		return checkUInt64SystemVar(name, value, 0, 4294967295, vars)
	case OldPasswords:
		return checkUInt64SystemVar(name, value, 0, 2, vars)
	case SessionTrackGtids:
		if strings.EqualFold(value, "OFF") || value == "0" {
			return "OFF", nil
		} else if strings.EqualFold(value, "OWN_GTID") || value == "1" {
			return "OWN_GTID", nil
		} else if strings.EqualFold(value, "ALL_GTIDS") || value == "2" {
			return "ALL_GTIDS", nil
		}
		return value, ErrWrongValueForVar.GenWithStackByArgs(name, value)
	case SQLSelectLimit:
		return checkUInt64SystemVar(name, value, 0, math.MaxUint64, vars)
	case SyncBinlog:
		return checkUInt64SystemVar(name, value, 0, 4294967295, vars)
	case TableDefinitionCache:
		return checkUInt64SystemVar(name, value, 400, 524288, vars)
	case TmpTableSize:
		return checkUInt64SystemVar(name, value, 1024, math.MaxUint64, vars)
	case TimeZone:
		if strings.EqualFold(value, "SYSTEM") {
			return "SYSTEM", nil
		}
		return value, nil
	case WarningCount, ErrorCount:
		return value, ErrReadOnly.GenWithStackByArgs(name)
	case GeneralLog, TiDBGeneralLog, AvoidTemporalUpgrade, BigTables, CheckProxyUsers, CoreFile, EndMakersInJSON, SQLLogBin, OfflineMode,
		PseudoSlaveMode, LowPriorityUpdates, SkipNameResolve, ForeignKeyChecks, SQLSafeUpdates, TiDBConstraintCheckInPlace:
		if strings.EqualFold(value, "ON") || value == "1" {
			return "1", nil
		} else if strings.EqualFold(value, "OFF") || value == "0" {
			return "0", nil
		}
		return value, ErrWrongValueForVar.GenWithStackByArgs(name, value)
	case AutocommitVar, TiDBSkipUTF8Check, TiDBOptAggPushDown,
		TiDBOptInSubqToJoinAndAgg,
		TiDBBatchInsert, TiDBDisableTxnAutoRetry, TiDBEnableStreaming,
		TiDBBatchDelete, TiDBEnableCascadesPlanner:
		if strings.EqualFold(value, "ON") || value == "1" || strings.EqualFold(value, "OFF") || value == "0" {
			return value, nil
		}
		return value, ErrWrongValueForVar.GenWithStackByArgs(name, value)
	case TiDBEnableTablePartition:
		switch {
		case strings.EqualFold(value, "ON") || value == "1":
			return "on", nil
		case strings.EqualFold(value, "OFF") || value == "0":
			return "off", nil
		case strings.EqualFold(value, "AUTO"):
			return "auto", nil
		}
		return value, ErrWrongValueForVar.GenWithStackByArgs(name, value)
	case TiDBIndexLookupConcurrency, TiDBIndexLookupJoinConcurrency, TiDBIndexJoinBatchSize,
		TiDBIndexLookupSize,
		TiDBHashJoinConcurrency,
		TiDBHashAggPartialConcurrency,
		TiDBHashAggFinalConcurrency,
		TiDBDistSQLScanConcurrency,
		TiDBIndexSerialScanConcurrency, TiDBDDLReorgWorkerCount,
		TiDBBackoffLockFast, TiDBMaxChunkSize,
		TiDBDMLBatchSize, TiDBOptimizerSelectivityLevel:
		v, err := strconv.Atoi(value)
		if err != nil {
			return value, ErrWrongTypeForVar.GenWithStackByArgs(name)
		}
		if v <= 0 {
			return value, ErrWrongValueForVar.GenWithStackByArgs(name, value)
		}
		return value, nil
	case TiDBProjectionConcurrency,
		TIDBMemQuotaQuery,
		TIDBMemQuotaHashJoin,
		TIDBMemQuotaMergeJoin,
		TIDBMemQuotaSort,
		TIDBMemQuotaTopn,
		TIDBMemQuotaIndexLookupReader,
		TIDBMemQuotaIndexLookupJoin,
		TIDBMemQuotaNestedLoopApply,
		TiDBRetryLimit,
		TiDBSlowLogThreshold,
		TiDBQueryLogMaxLen:
		_, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return value, ErrWrongValueForVar.GenWithStackByArgs(name)
		}
		return value, nil
	case TiDBAutoAnalyzeStartTime, TiDBAutoAnalyzeEndTime:
		v, err := setAnalyzeTime(vars, value)
		if err != nil {
			return "", errors.Trace(err)
		}
		return v, nil
	case TxnIsolation, TransactionIsolation:
		upVal := strings.ToUpper(value)
		_, exists := TxIsolationNames[upVal]
		if !exists {
			return "", ErrWrongValueForVar.GenWithStackByArgs(name, value)
		}
		return upVal, nil
	}
	return value, nil
}

// TiDBOptOn could be used for all tidb session variable options, we use "ON"/1 to turn on those options.
func TiDBOptOn(opt string) bool {
	return strings.EqualFold(opt, "ON") || opt == "1"
}

func tidbOptPositiveInt32(opt string, defaultVal int) int {
	val, err := strconv.Atoi(opt)
	if err != nil || val <= 0 {
		return defaultVal
	}
	return val
}

func tidbOptInt64(opt string, defaultVal int64) int64 {
	val, err := strconv.ParseInt(opt, 10, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

func parseTimeZone(s string) (*time.Location, error) {
	if strings.EqualFold(s, "SYSTEM") {
		return timeutil.SystemLocation(), nil
	}

	loc, err := time.LoadLocation(s)
	if err == nil {
		return loc, nil
	}

	// The value can be given as a string indicating an offset from UTC, such as '+10:00' or '-6:00'.
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		d, err := types.ParseDuration(nil, s[1:], 0)
		if err == nil {
			ofst := int(d.Duration / time.Second)
			if s[0] == '-' {
				ofst = -ofst
			}
			return time.FixedZone("", ofst), nil
		}
	}

	return nil, ErrUnknownTimeZone.GenWithStackByArgs(s)
}

func setSnapshotTS(s *SessionVars, sVal string) error {
	if sVal == "" {
		s.SnapshotTS = 0
		return nil
	}

	if tso, err := strconv.ParseUint(sVal, 10, 64); err == nil {
		s.SnapshotTS = tso
		return nil
	}

	t, err := types.ParseTime(s.StmtCtx, sVal, mysql.TypeTimestamp, types.MaxFsp)
	if err != nil {
		return errors.Trace(err)
	}

	// TODO: Consider time_zone variable.
	t1, err := t.Time.GoTime(time.Local)
	s.SnapshotTS = GoTimeToTS(t1)
	return errors.Trace(err)
}

// GoTimeToTS converts a Go time to uint64 timestamp.
func GoTimeToTS(t time.Time) uint64 {
	ts := (t.UnixNano() / int64(time.Millisecond)) << epochShiftBits
	return uint64(ts)
}

const (
	analyzeLocalTimeFormat = "15:04"
	// AnalyzeFullTimeFormat is the full format of analyze start time and end time.
	AnalyzeFullTimeFormat = "15:04 -0700"
)

func setAnalyzeTime(s *SessionVars, val string) (string, error) {
	var t time.Time
	var err error
	if len(val) <= len(analyzeLocalTimeFormat) {
		t, err = time.ParseInLocation(analyzeLocalTimeFormat, val, s.TimeZone)
	} else {
		t, err = time.ParseInLocation(AnalyzeFullTimeFormat, val, s.TimeZone)
	}
	if err != nil {
		return "", errors.Trace(err)
	}
	return t.Format(AnalyzeFullTimeFormat), nil
}
