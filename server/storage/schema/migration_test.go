// Copyright 2021 The etcd Authors
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

package schema

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

func TestNewPlan(t *testing.T) {
	tcs := []struct {
		name string

		current semver.Version
		target  semver.Version

		expectError    bool
		expectErrorMsg string
	}{
		{
			name:    "Update v3.5 to v3.6 should work",
			current: version.V3_5,
			target:  version.V3_6,
		},
		{
			name:    "Downgrade v3.6 to v3.5 should fail as downgrades are not yet supported",
			current: version.V3_6,
			target:  version.V3_5,
		},
		{
			name:           "Upgrade v3.6 to v3.7 should fail as v3.7 is unknown",
			current:        version.V3_6,
			target:         version.V3_7,
			expectError:    true,
			expectErrorMsg: `version "3.7.0" is not supported`,
		},
		{
			name:           "Upgrade v3.6 to v4.0 as major version changes are unsupported",
			current:        version.V3_6,
			target:         version.V4_0,
			expectError:    true,
			expectErrorMsg: "changing major storage version is not supported",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			_, err := newPlan(lg, tc.current, tc.target)
			if (err != nil) != tc.expectError {
				t.Errorf("newPlan(lg, %q, %q) returned unexpected error (or lack thereof), expected: %v, got: %v", tc.current, tc.target, tc.expectError, err)
			}
			if err != nil && err.Error() != tc.expectErrorMsg {
				t.Errorf("newPlan(lg, %q, %q) returned unexpected error message, expected: %q, got: %q", tc.current, tc.target, tc.expectErrorMsg, err.Error())
			}
		})
	}
}

func TestMigrationStepExecute(t *testing.T) {
	recorder := &actionRecorder{}
	errorC := fmt.Errorf("error C")
	tcs := []struct {
		name string

		currentVersion semver.Version
		isUpgrade      bool
		changes        []schemaChange

		expectError           error
		expectVersion         *semver.Version
		expectRecordedActions []string
	}{
		{
			name:           "Upgrade execute changes in order and updates version",
			currentVersion: semver.Version{Major: 99, Minor: 0},
			isUpgrade:      true,
			changes: []schemaChange{
				recorder.changeMock("A"),
				recorder.changeMock("B"),
			},

			expectVersion:         &semver.Version{Major: 99, Minor: 1},
			expectRecordedActions: []string{"upgrade A", "upgrade B"},
		},
		{
			name:           "Downgrade execute changes in reversed order and downgrades version",
			currentVersion: semver.Version{Major: 99, Minor: 1},
			isUpgrade:      false,
			changes: []schemaChange{
				recorder.changeMock("A"),
				recorder.changeMock("B"),
			},

			expectVersion:         &semver.Version{Major: 99, Minor: 0},
			expectRecordedActions: []string{"downgrade B", "downgrade A"},
		},
		{
			name:           "Failure during upgrade should revert previous changes in reversed order and not change version",
			currentVersion: semver.Version{Major: 99, Minor: 0},
			isUpgrade:      true,
			changes: []schemaChange{
				recorder.changeMock("A"),
				recorder.changeMock("B"),
				recorder.changeError(errorC),
				recorder.changeMock("D"),
				recorder.changeMock("E"),
			},

			expectVersion:         &semver.Version{Major: 99, Minor: 0},
			expectRecordedActions: []string{"upgrade A", "upgrade B", "upgrade error C", "revert upgrade B", "revert upgrade A"},
			expectError:           errorC,
		},
		{
			name:           "Failure during downgrade should revert previous changes in reversed order and not change version",
			currentVersion: semver.Version{Major: 99, Minor: 0},
			isUpgrade:      false,
			changes: []schemaChange{
				recorder.changeMock("A"),
				recorder.changeMock("B"),
				recorder.changeError(errorC),
				recorder.changeMock("D"),
				recorder.changeMock("E"),
			},

			expectVersion:         &semver.Version{Major: 99, Minor: 0},
			expectRecordedActions: []string{"downgrade E", "downgrade D", "downgrade error C", "revert downgrade D", "revert downgrade E"},
			expectError:           errorC,
		},
		{
			name:           "Downgrade below to below v3.6 doesn't leave storage version as it was not supported then",
			currentVersion: semver.Version{Major: 3, Minor: 6},
			changes:        schemaChanges[version.V3_6],
			isUpgrade:      false,
			expectVersion:  nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			recorder.actions = []string{}
			if tc.expectRecordedActions == nil {
				tc.expectRecordedActions = []string{}
			}
			lg := zaptest.NewLogger(t)

			be, _ := betesting.NewTmpBackend(t, time.Microsecond, 10)
			defer be.Close()
			tx := be.BatchTx()
			if tx == nil {
				t.Fatal("batch tx is nil")
			}
			tx.Lock()
			defer tx.Unlock()

			UnsafeCreateMetaBucket(tx)
			UnsafeSetStorageVersion(tx, &tc.currentVersion)

			step := newMigrationStep(tc.currentVersion, tc.isUpgrade, tc.changes)
			err := step.unsafeExecute(lg, tx)
			if err != tc.expectError {
				t.Errorf("Unexpected error or lack thereof, expected: %v, got: %v", tc.expectError, err)
			}
			v := UnsafeReadStorageVersion(tx)
			assert.Equal(t, tc.expectVersion, v)
			assert.Equal(t, tc.expectRecordedActions, recorder.actions)
		})
	}
}

type actionRecorder struct {
	actions []string
}

func (r *actionRecorder) changeMock(name string) schemaChange {
	return changeMock(r, name, nil)
}

func (r *actionRecorder) changeError(err error) schemaChange {
	return changeMock(r, fmt.Sprintf("%v", err), err)
}

func changeMock(recorder *actionRecorder, name string, err error) schemaChange {
	return simpleSchemaChange{
		upgrade: actionMock{
			recorder: recorder,
			name:     "upgrade " + name,
			err:      err,
		},
		downgrade: actionMock{
			recorder: recorder,
			name:     "downgrade " + name,
			err:      err,
		},
	}
}

type actionMock struct {
	recorder *actionRecorder
	name     string
	err      error
}

func (a actionMock) unsafeDo(tx backend.UnsafeReadWriter) (action, error) {
	a.recorder.actions = append(a.recorder.actions, a.name)
	return actionMock{
		recorder: a.recorder,
		name:     "revert " + a.name,
	}, a.err
}
