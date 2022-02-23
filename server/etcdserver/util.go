// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdserver

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"

	"go.uber.org/zap"
)

// isConnectedToQuorumSince checks whether the local member is connected to the
// quorum of the cluster since the given time.
func isConnectedToQuorumSince(transport rafthttp.Transporter, since time.Time, self types.ID, members []*membership.Member) bool {
	return numConnectedSince(transport, since, self, members) >= (len(members)/2)+1
}

// isConnectedSince checks whether the local member is connected to the
// remote member since the given time.
func isConnectedSince(transport rafthttp.Transporter, since time.Time, remote types.ID) bool {
	t := transport.ActiveSince(remote)
	return !t.IsZero() && t.Before(since)
}

// isConnectedFullySince checks whether the local member is connected to all
// members in the cluster since the given time.
func isConnectedFullySince(transport rafthttp.Transporter, since time.Time, self types.ID, members []*membership.Member) bool {
	return numConnectedSince(transport, since, self, members) == len(members)
}

// numConnectedSince counts how many members are connected to the local member
// since the given time.
func numConnectedSince(transport rafthttp.Transporter, since time.Time, self types.ID, members []*membership.Member) int {
	connectedNum := 0
	for _, m := range members {
		if m.ID == self || isConnectedSince(transport, since, m.ID) {
			connectedNum++
		}
	}
	return connectedNum
}

// longestConnected chooses the member with longest active-since-time.
// It returns false, if nothing is active.
func longestConnected(tp rafthttp.Transporter, membs []types.ID) (types.ID, bool) {
	var longest types.ID
	var oldest time.Time
	for _, id := range membs {
		tm := tp.ActiveSince(id)
		if tm.IsZero() { // inactive
			continue
		}

		if oldest.IsZero() { // first longest candidate
			oldest = tm
			longest = id
		}

		if tm.Before(oldest) {
			oldest = tm
			longest = id
		}
	}
	if uint64(longest) == 0 {
		return longest, false
	}
	return longest, true
}

type notifier struct {
	c   chan struct{}
	err error
}

func newNotifier() *notifier {
	return &notifier{
		c: make(chan struct{}),
	}
}

func (nc *notifier) notify(err error) {
	nc.err = err
	close(nc.c)
}

func warnOfExpensiveRequest(lg *zap.Logger, warningApplyDuration time.Duration, tr *TimeRecorder, reqStringer fmt.Stringer, respMsg proto.Message, err error) {
	if tr.TotalDuration() <= warningApplyDuration {
		return
	}
	var resp string
	if !isNil(respMsg) {
		resp = fmt.Sprintf("size:%d", proto.Size(respMsg))
	}
	warnOfExpensiveGenericRequest(lg, warningApplyDuration, tr, reqStringer, "", resp, err)
}

func warnOfFailedRequest(lg *zap.Logger, now time.Time, reqStringer fmt.Stringer, respMsg proto.Message, err error) {
	var resp string
	if !isNil(respMsg) {
		resp = fmt.Sprintf("size:%d", proto.Size(respMsg))
	}
	d := time.Since(now)
	lg.Warn(
		"failed to apply request",
		zap.Duration("took", d),
		zap.String("request", reqStringer.String()),
		zap.String("response", resp),
		zap.Error(err),
	)
}

func warnOfExpensiveReadOnlyTxnRequest(lg *zap.Logger, warningApplyDuration time.Duration, tr *TimeRecorder, r *pb.TxnRequest, txnResponse *pb.TxnResponse, err error) {
	if tr.TotalDuration() <= warningApplyDuration {
		return
	}
	reqStringer := pb.NewLoggableTxnRequest(r)
	var resp string
	if !isNil(txnResponse) {
		var resps []string
		for _, r := range txnResponse.Responses {
			switch op := r.Response.(type) {
			case *pb.ResponseOp_ResponseRange:
				resps = append(resps, fmt.Sprintf("range_response_count:%d", len(op.ResponseRange.Kvs)))
			default:
				// only range responses should be in a read only txn request
			}
		}
		resp = fmt.Sprintf("responses:<%s> size:%d", strings.Join(resps, " "), txnResponse.Size())
	}
	warnOfExpensiveGenericRequest(lg, warningApplyDuration, tr, reqStringer, "read-only txn ", resp, err)
}

func warnOfExpensiveReadOnlyRangeRequest(lg *zap.Logger, warningApplyDuration time.Duration, tr *TimeRecorder, reqStringer fmt.Stringer, rangeResponse *pb.RangeResponse, err error) {
	if tr.TotalDuration() <= warningApplyDuration {
		return
	}
	var resp string
	if !isNil(rangeResponse) {
		resp = fmt.Sprintf("range_response_count:%d size:%d", len(rangeResponse.Kvs), rangeResponse.Size())
	}
	warnOfExpensiveGenericRequest(lg, warningApplyDuration, tr, reqStringer, "read-only range ", resp, err)
}

// callers need make sure time has passed warningApplyDuration
func warnOfExpensiveGenericRequest(lg *zap.Logger, warningApplyDuration time.Duration, tr *TimeRecorder, reqStringer fmt.Stringer, prefix string, resp string, err error) {
	lg.Warn(
		"apply request took too long",
		zap.String("took", tr.String()),
		zap.Duration("expected-duration", warningApplyDuration),
		zap.String("prefix", prefix),
		zap.String("request", reqStringer.String()),
		zap.String("response", resp),
		zap.Error(err),
	)
	slowApplies.Inc()
}

func isNil(msg proto.Message) bool {
	return msg == nil || reflect.ValueOf(msg).IsNil()
}

// panicAlternativeStringer wraps a fmt.Stringer, and if calling String() panics, calls the alternative instead.
// This is needed to ensure logging slow v2 requests does not panic, which occurs when running integration tests
// with the embedded server with github.com/golang/protobuf v1.4.0+. See https://github.com/etcd-io/etcd/issues/12197.
type panicAlternativeStringer struct {
	stringer    fmt.Stringer
	alternative func() string
}

func (n panicAlternativeStringer) String() (s string) {
	defer func() {
		if err := recover(); err != nil {
			s = n.alternative()
		}
	}()
	s = n.stringer.String()
	return s
}

type TimeRecorder struct {
	// start is the earliest time point.
	start time.Time
	// timeMap saves some time points, each of which has a name (the key).
	timeMap map[string]time.Time
}

// NewTimeRecorder creates an instance of TimeRecorder
func NewTimeRecorder() *TimeRecorder {
	return &TimeRecorder{
		start:   time.Now(),
		timeMap: make(map[string]time.Time),
	}
}

// GetStart returns the start time.
func (tr *TimeRecorder) GetStart() time.Time {
	return tr.start
}

// SetStart sets a start time, by default it's TimeRecorder's creation time.
func (tr *TimeRecorder) SetStart(t time.Time) {
	tr.start = t
}

// AddTime saves the time with name into the map.
func (tr *TimeRecorder) AddTime(name string, t time.Time) {
	tr.timeMap[name] = t
}

func (tr *TimeRecorder) TotalDuration() time.Duration {
	return time.Since(tr.start)
}

func (tr *TimeRecorder) String() string {
	var timeDurations []string

	rightNow := time.Now()

	timeDurations = append(timeDurations, fmt.Sprintf("total: %s", rightNow.Sub(tr.start)))

	for k, v := range tr.timeMap {
		timeDurations = append(timeDurations, fmt.Sprintf("%s: %s", k, rightNow.Sub(v)))
	}

	return strings.Join(timeDurations, ",")
}
