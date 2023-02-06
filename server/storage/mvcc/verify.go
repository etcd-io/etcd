// Copyright 2023 The etcd Authors
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

package mvcc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/verify"
)

const (
	ENV_VERIFY_WATCH_EVENT verify.VerificationType = "watchEvent"
)

var (
	lastWatchRev    int64
	lastWatchEvents []mvccpb.Event
)

func VerifyWatchEvents(lg *zap.Logger, rev int64, evs []mvccpb.Event) {
	if !verifyWatchEnabled() {
		return
	}

	// Revision should never decrease.
	if rev <= lastWatchRev && lastWatchRev != 0 {
		failWatchVerification(lg, "Received a smaller revision", rev, evs)
	}

	// Each ModRevision must be equal to the `rev`.
	for i, ev := range evs {
		if ev.Kv.ModRevision != rev {
			failWatchVerification(lg, fmt.Sprintf("events[%d].Kv.ModRevision doesn't match rev", i), rev, evs)
		}
	}

	// There shouldn't have any duplicated events as compared to the last watch events.
	for _, oldEvt := range lastWatchEvents {
		for _, newEvt := range evs {
			if oldEvt.Type == newEvt.Type && reflect.DeepEqual(oldEvt, newEvt) {
				failWatchVerification(lg, "Found duplicated events", rev, evs)
			}
		}
	}

	lastWatchRev = rev
	lastWatchEvents = evs
}

func verifyWatchEnabled() bool {
	return verify.IsVerificationEnabled(ENV_VERIFY_WATCH_EVENT)
}

func failWatchVerification(lg *zap.Logger, msg string, rev int64, evs []mvccpb.Event) {
	marshaledLastEvents, err := json.MarshalIndent(lastWatchEvents, "", " ")
	if err != nil {
		lg.Error("Failed to marshal lastWatchEvents", zap.Error(err))
	}

	marshaledCurrentEvents, err := json.MarshalIndent(evs, "", " ")
	if err != nil {
		lg.Error("Failed to marshal current events", zap.Error(err))
	}

	var sb strings.Builder
	sb.WriteString(msg + "\n")
	sb.WriteString(fmt.Sprintf("lastRev: %d\n", lastWatchRev))
	sb.WriteString(fmt.Sprintf("curRev: %d\n", rev))

	sb.WriteString("------\n")
	sb.WriteString("lastWatchEvents:\n")
	sb.Write(marshaledLastEvents)
	sb.WriteString("\n")

	sb.WriteString("------\n")
	sb.WriteString("currentWatchEvents:\n")
	sb.Write(marshaledCurrentEvents)
	sb.WriteString("\n")

	panic(sb.String())
}
