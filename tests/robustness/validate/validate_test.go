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

//nolint:unparam
package validate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/tests/v3/framework/testutils"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func TestDataReports(t *testing.T) {
	testdataPath := testutils.MustAbsPath("../testdata/")
	files, err := os.ReadDir(testdataPath)
	assert.NoError(t, err)
	for _, file := range files {
		if file.Name() == ".gitignore" {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			path := filepath.Join(testdataPath, file.Name())
			reports, err := report.LoadClientReports(path)
			assert.NoError(t, err)

			persistedRequests, err := report.LoadClusterPersistedRequests(lg, path)
			if err != nil {
				t.Fatal(err)
			}
			visualize := ValidateAndReturnVisualize(t, zaptest.NewLogger(t), Config{}, reports, persistedRequests, 5*time.Minute)

			if t.Failed() {
				err := visualize(filepath.Join(path, "history.html"))
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestValidateWatch(t *testing.T) {
	tcs := []struct {
		name         string
		config       Config
		reports      []report.ClientReport
		eventHistory []model.PersistedEvent
		expectError  string
	}{
		{
			name: "Ordered, Unique - ordered unique events in one response - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 3, true),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Ordered, Unique - unique ordered events in separate response - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 3, true),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Ordered - unordered events in one response - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 3, true),
										putWatchEvent("a", "1", 2, true),
									},
								},
							},
						},
					},
				},
			},
			expectError: errBrokeOrdered.Error(),
		},
		{
			name: "Ordered - unordered events in separate response - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 3, true),
									},
								},
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
							},
						},
					},
				},
			},
			expectError: errBrokeOrdered.Error(),
		},
		{
			name: "Ordered - unordered events in separate watch - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "b",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 3, true),
									},
								},
							},
						},
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Unique - duplicated events in one response - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("a", "2", 2, true),
									},
								},
							},
						},
					},
				},
			},
			expectError: errBrokeUnique.Error(),
		},
		{
			name: "Unique - duplicated events in separate responses - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "2", 2, true),
									},
								},
							},
						},
					},
				},
			},
			expectError: errBrokeUnique.Error(),
		},
		{
			name: "Unique - duplicated events in watch requests - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
							},
						},
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "2", 2, true),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Unique, Atomic - duplicated revision in one response - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 2, true),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Unique - duplicated revision in separate watch request - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
							},
						},
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name:   "Unique revision - duplicated revision in one response - fail",
			config: Config{ExpectRevisionUnique: true},
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 2, true),
									},
								},
							},
						},
					},
				},
			},
			expectError: errBrokeUnique.Error(),
		},
		{
			name:   "Atomic - duplicated revision in one response - fail",
			config: Config{ExpectRevisionUnique: true},
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 2, true),
									},
								},
							},
						},
					},
				},
			},
			expectError: errBrokeUnique.Error(),
		},
		{
			name: "Atomic - revision in separate responses - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 2, true),
									},
								},
							},
						},
					},
				},
			},
			expectError: errBrokeAtomic.Error(),
		},
		{
			name: "Resumable, Reliable, Bookmarkable - all events with bookmark - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 3, true),
										putWatchEvent("c", "3", 4, true),
									},
								},
								{
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
		},
		{
			name: "Resumable, Reliable, Bookmarkable - empty events without revision - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
		},
		{
			name: "Resumable, Reliable, Bookmarkable - empty events with watch revision - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Revision:   2,
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
			expectError: errBrokeBookmarkable.Error(),
		},
		{
			name: "Bookmarkable - revision non decreasing - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Revision:         1,
									IsProgressNotify: true,
								},
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
								{
									Revision:         2,
									IsProgressNotify: true,
								},
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 3, true),
									},
								},
								{
									Revision:         3,
									IsProgressNotify: true,
								},
								{
									Revision:         3,
									IsProgressNotify: true,
								},
								{
									Events: []model.WatchEvent{
										putWatchEvent("c", "3", 4, true),
									},
								},
								{
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
		},
		{
			name: "Bookmarkable - event precedes progress - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 3, true),
										putWatchEvent("c", "3", 4, true),
									},
								},
								{
									Revision:         3,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
			expectError: errBrokeBookmarkable.Error(),
		},
		{
			name: "Bookmarkable - progress precedes event - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 3, true),
									},
								},
								{
									Revision:         4,
									IsProgressNotify: true,
								},
								{
									Events: []model.WatchEvent{
										putWatchEvent("c", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
			expectError: errBrokeBookmarkable.Error(),
		},
		{
			name: "Bookmarkable - missing event before bookmark - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 3, true),
									},
								},
								{
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
			expectError: errBrokeBookmarkable.Error(),
		},
		{
			name: "Bookmarkable - missing event matching watch before bookmark - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
									},
								},
								{
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				putPersistedEvent("c", "3", 4, true),
			},
			expectError: errBrokeBookmarkable.Error(),
		},
		{
			name: "Bookmarkable - missing event matching watch with prefix before bookmark - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "a",
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("aa", "1", 2, true),
									},
								},
								{
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("aa", "1", 2, true),
				putPersistedEvent("ab", "2", 3, true),
				putPersistedEvent("cc", "3", 4, true),
			},
			expectError: errBrokeBookmarkable.Error(),
		},
		{
			name: "Reliable - all events history - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 3, true),
										putWatchEvent("c", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
			expectError: "",
		},
		{
			name: "Reliable - missing middle event - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("c", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
			expectError: errBrokeReliable.Error(),
		},
		{
			name: "Reliable - middle event doesn't match request - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("a", "3", 4, false),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("ab", "2", 3, true),
				putPersistedEvent("a", "3", 4, false),
			},
		},
		{
			name: "Reliable - middle event doesn't match request with prefix - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "a",
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("aa", "1", 2, true),
										putWatchEvent("ac", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("aa", "1", 2, true),
				putPersistedEvent("bb", "2", 3, true),
				putPersistedEvent("ac", "3", 4, true),
			},
		},
		{
			name: "Reliable, Resumable - missing first event - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 3, true),
										putWatchEvent("c", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
			// TODO: Should pass as watch with revision 0 might start from any revision.
			expectError: errBrokeResumable.Error(),
		},
		{
			name: "Reliable - missing last event - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 3, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
		},
		{
			name: "Resumable - watch revision from middle event - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
								Revision:   3,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 3, true),
										putWatchEvent("c", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
		},
		{
			name: "Resumable - watch key from middle event - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:      "b",
								Revision: 2,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "2", 3, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
		},
		{
			name: "Resumable - watch key with prefix from middle event - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "b",
								Revision:   2,
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("bb", "2", 3, true),
										putWatchEvent("bc", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("bb", "2", 3, true),
				putPersistedEvent("bc", "3", 4, true),
			},
		},
		{
			name: "Resumable - missing first matching event - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
								Revision:   3,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("c", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("c", "3", 4, true),
			},
			expectError: errBrokeResumable.Error(),
		},
		{
			name: "Resumable - missing first matching event with prefix - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:      "b",
								Revision: 2,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("b", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("b", "3", 4, true),
			},
			expectError: errBrokeResumable.Error(),
		},
		{
			name: "Resumable - missing first matching event with prefix - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "b",
								WithPrefix: true,
								Revision:   2,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("bc", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("bb", "2", 3, true),
				putPersistedEvent("bc", "3", 4, true),
			},
			expectError: errBrokeResumable.Error(),
		},
		{
			name: "IsCreate - correct IsCreate values - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("a", "2", 3, false),
										deleteWatchEvent("a", 4),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
		},
		{
			name: "IsCreate - second put marked as created - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("a", "2", 3, true),
										deleteWatchEvent("a", 4),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
			expectError: errBrokeIsCreate.Error(),
		},
		{
			name: "IsCreate - put after delete marked as not created - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("a", "2", 3, false),
										deleteWatchEvent("a", 4),
										putWatchEvent("a", "4", 5, false),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
			expectError: errBrokeIsCreate.Error(),
		},
		{
			name: "PrevKV - no previous values - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "a",
								WithPrevKV: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("a", "2", 3, false),
										deleteWatchEvent("a", 4),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
		},
		{
			name: "PrevKV - all previous values - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "a",
								WithPrevKV: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEventWithPrevKV("a", "2", 3, false, "1", 2),
										deleteWatchEventWithPrevKV("a", 4, "2", 3),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
		},
		{
			name: "PrevKV - mismatch value on put - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "a",
								WithPrevKV: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEventWithPrevKV("a", "2", 3, false, "2", 2),
										deleteWatchEventWithPrevKV("a", 4, "2", 3),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
			expectError: errBrokePrevKV.Error(),
		},
		{
			name: "PrevKV - mismatch revision on put - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "a",
								WithPrevKV: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEventWithPrevKV("a", "2", 3, false, "1", 3),
										deleteWatchEventWithPrevKV("a", 4, "2", 3),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
			expectError: errBrokePrevKV.Error(),
		},
		{
			name: "PrevKV - mismatch value on delete - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "a",
								WithPrevKV: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEventWithPrevKV("a", "2", 3, false, "1", 2),
										deleteWatchEventWithPrevKV("a", 4, "1", 3),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
			expectError: errBrokePrevKV.Error(),
		},
		{
			name: "PrevKV - mismatch revision on delete - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
								WithPrevKV: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEventWithPrevKV("a", "2", 3, false, "1", 2),
										deleteWatchEventWithPrevKV("a", 4, "2", 2),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("a", "2", 3, false),
				deletePersistedEvent("a", 4),
				putPersistedEvent("a", "4", 5, true),
			},
			expectError: errBrokePrevKV.Error(),
		},
		{
			name: "Filter - event not matching the watch - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "a",
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("a", "1", 2, true),
										putWatchEvent("b", "2", 3, true),
										putWatchEvent("a", "3", 4, false),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("a", "1", 2, true),
				putPersistedEvent("b", "2", 3, true),
				putPersistedEvent("a", "3", 4, false),
			},
			expectError: errBrokeFilter.Error(),
		},
		{
			name: "Filter - event not matching the watch with prefix - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:        "a",
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										putWatchEvent("aa", "1", 2, true),
										putWatchEvent("bb", "2", 3, true),
										putWatchEvent("ac", "3", 4, true),
									},
								},
							},
						},
					},
				},
			},
			eventHistory: []model.PersistedEvent{
				putPersistedEvent("aa", "1", 2, true),
				putPersistedEvent("bb", "2", 3, true),
				putPersistedEvent("ac", "3", 4, true),
			},
			expectError: errBrokeFilter.Error(),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWatch(zaptest.NewLogger(t), tc.config, tc.reports, tc.eventHistory)
			var errStr string
			if err != nil {
				errStr = err.Error()
			}
			if errStr != tc.expectError {
				t.Errorf("validateWatch(...), got: %q, want: %q", err, tc.expectError)
			}
		})
	}
}

func putWatchEvent(key, value string, rev int64, isCreate bool) model.WatchEvent {
	return model.WatchEvent{
		PersistedEvent: putPersistedEvent(key, value, rev, isCreate),
	}
}

func deleteWatchEvent(key string, rev int64) model.WatchEvent {
	return model.WatchEvent{
		PersistedEvent: deletePersistedEvent(key, rev),
	}
}

func putWatchEventWithPrevKV(key, value string, rev int64, isCreate bool, prevValue string, modRev int64) model.WatchEvent {
	return model.WatchEvent{
		PersistedEvent: putPersistedEvent(key, value, rev, isCreate),
		PrevValue: &model.ValueRevision{
			Value:       model.ToValueOrHash(prevValue),
			ModRevision: modRev,
		},
	}
}

func deleteWatchEventWithPrevKV(key string, rev int64, prevValue string, modRev int64) model.WatchEvent {
	return model.WatchEvent{
		PersistedEvent: deletePersistedEvent(key, rev),
		PrevValue: &model.ValueRevision{
			Value:       model.ToValueOrHash(prevValue),
			ModRevision: modRev,
		},
	}
}

func putPersistedEvent(key, value string, rev int64, isCreate bool) model.PersistedEvent {
	return model.PersistedEvent{
		Event: model.Event{
			Type:  model.PutOperation,
			Key:   key,
			Value: model.ToValueOrHash(value),
		},
		Revision: rev,
		IsCreate: isCreate,
	}
}

func deletePersistedEvent(key string, rev int64) model.PersistedEvent {
	return model.PersistedEvent{
		Event: model.Event{
			Type: model.DeleteOperation,
			Key:  key,
		},
		Revision: rev,
	}
}
