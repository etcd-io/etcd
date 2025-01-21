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

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/tests/v3/framework/testutils"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func TestDataReports(t *testing.T) {
	testdataPath := testutils.MustAbsPath("../testdata/")
	files, err := os.ReadDir(testdataPath)
	require.NoError(t, err)
	for _, file := range files {
		if file.Name() == ".gitignore" {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			lg := zaptest.NewLogger(t)
			path := filepath.Join(testdataPath, file.Name())
			reports, err := report.LoadClientReports(path)
			require.NoError(t, err)

			persistedRequests, err := report.LoadClusterPersistedRequests(lg, path)
			require.NoError(t, err)
			visualize := ValidateAndReturnVisualize(t, zaptest.NewLogger(t), Config{}, reports, persistedRequests, 5*time.Minute)

			err = visualize(filepath.Join(path, "history.html"))
			require.NoError(t, err)
		})
	}
}

func TestValidateWatch(t *testing.T) {
	tcs := []struct {
		name              string
		config            Config
		reports           []report.ClientReport
		persistedRequests []model.EtcdRequest
		expectError       string
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
			},
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
										putWatchEvent("a", "1", 2, true),
									},
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
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
			persistedRequests: []model.EtcdRequest{
				{
					Type:        model.Txn,
					LeaseGrant:  nil,
					LeaseRevoke: nil,
					Range:       nil,
					Txn: &model.TxnRequest{
						Conditions: nil,
						OperationsOnSuccess: []model.EtcdOperation{
							{
								Type: model.PutOperation,
								Put: model.PutOptions{
									Key:   "a",
									Value: model.ToValueOrHash("1"),
								},
							},
							{
								Type: model.PutOperation,
								Put: model.PutOptions{
									Key:   "b",
									Value: model.ToValueOrHash("2"),
								},
							},
						},
						OperationsOnFailure: nil,
					},
					Defragment: nil,
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
			},
			expectError: errBrokeAtomic.Error(),
		},
		{
			name: "Resumable, Reliable, Bookmarkable - all events with watch revision and bookmark - pass",
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
		},
		{
			name: "Resumable, Reliable, Bookmarkable - all events with only bookmarks - pass",
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			expectError: errBrokeReliable.Error(),
		},
		{
			name: "Resumable, Reliable, Bookmarkable - unmatched events with watch revision - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:      "d",
								Revision: 2,
							},
							Responses: []model.WatchResponse{
								{
									Revision:         2,
									IsProgressNotify: true,
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
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
		},
		{
			name: "Resumable, Reliable, Bookmarkable - empty events between progress notifies - fail",
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
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			expectError: errBrokeReliable.Error(),
		},
		{
			name: "Resumable, Reliable, Bookmarkable - unmatched events between progress notifies - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key: "d",
							},
							Responses: []model.WatchResponse{
								{
									Revision:         2,
									IsProgressNotify: true,
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
									Revision:         4,
									IsProgressNotify: true,
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			expectError: errBrokeBookmarkable.Error(),
		},
		{
			name: "Bookmarkable - progress precedes other progress - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									IsProgressNotify: true,
									Revision:         2,
								},
								{
									IsProgressNotify: true,
									Revision:         1,
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{},
			expectError:       errBrokeBookmarkable.Error(),
		},
		{
			name: "Bookmarkable - progress notification lower than watch request - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Revision:   3,
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									IsProgressNotify: true,
									Revision:         2,
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
			},
		},
		{
			name: "Bookmarkable - empty event history - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
							},
							Responses: []model.WatchResponse{
								{
									IsProgressNotify: true,
									Revision:         1,
								},
								{
									IsProgressNotify: true,
									Revision:         1,
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{},
		},
		{
			name: "Reliable - missing event before bookmark - fail",
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			expectError: errBrokeReliable.Error(),
		},
		{
			name: "Reliable - missing event matching watch before bookmark - fail",
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				putRequest("c", "3"),
			},
			expectError: errBrokeReliable.Error(),
		},
		{
			name: "Reliable - missing event matching watch with prefix before bookmark - fail",
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
			persistedRequests: []model.EtcdRequest{
				putRequest("aa", "1"),
				putRequest("ab", "2"),
				putRequest("cc", "3"),
			},
			expectError: errBrokeReliable.Error(),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			expectError: "",
		},
		{
			name: "Reliable - single revision - pass",
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
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
			},
			expectError: "",
		},
		{
			name: "Reliable - single revision with watch revision - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
								Revision:   2,
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
			},
			expectError: "",
		},
		{
			name: "Reliable - missing single revision with watch revision - pass",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
								Revision:   2,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{},
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
			},
			expectError: "",
		},
		{
			name: "Reliable - single revision with progress notify - pass",
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
									IsProgressNotify: true,
									Revision:         2,
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
			},
			expectError: "",
		},
		{
			name: "Reliable - single revision missing with progress notify - fail",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								WithPrefix: true,
								Revision:   2,
							},
							Responses: []model.WatchResponse{
								{
									IsProgressNotify: true,
									Revision:         2,
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
			},
			expectError: errBrokeReliable.Error(),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("ab", "2"),
				putRequest("a", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("aa", "1"),
				putRequest("bb", "2"),
				putRequest("ac", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
		},
		{
			name: "Reliable - ignore empty last error response - pass",
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
									Revision: 5,
									Error:    "etcdserver: mvcc: required revision has been compacted",
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("bb", "2"),
				putRequest("bc", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
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
										putWatchEvent("b", "3", 4, false),
									},
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("b", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("bb", "2"),
				putRequest("bc", "3"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
										putWatchEventWithPrevKVV("a", "2", 3, false, "1", 2, 1),
										deleteWatchEventWithPrevKVV("a", 4, "2", 3, 2),
										putWatchEvent("a", "4", 5, true),
									},
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("a", "2"),
				deleteRequest("a"),
				putRequest("a", "4"),
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
									},
								},
							},
						},
					},
				},
			},
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
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
			persistedRequests: []model.EtcdRequest{
				putRequest("aa", "1"),
				putRequest("bb", "2"),
				putRequest("ac", "3"),
			},
			expectError: errBrokeFilter.Error(),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			replay := model.NewReplay(tc.persistedRequests)
			err := validateWatch(zaptest.NewLogger(t), tc.config, tc.reports, replay)
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
	return putWatchEventWithPrevKVV(key, value, rev, isCreate, prevValue, modRev, 0)
}

func putWatchEventWithPrevKVV(key, value string, rev int64, isCreate bool, prevValue string, modRev, ver int64) model.WatchEvent {
	return model.WatchEvent{
		PersistedEvent: putPersistedEvent(key, value, rev, isCreate),
		PrevValue: &model.ValueRevision{
			Value:       model.ToValueOrHash(prevValue),
			ModRevision: modRev,
			Version:     ver,
		},
	}
}

func deleteWatchEventWithPrevKV(key string, rev int64, prevValue string, modRev int64) model.WatchEvent {
	return deleteWatchEventWithPrevKVV(key, rev, prevValue, modRev, 0)
}

func deleteWatchEventWithPrevKVV(key string, rev int64, prevValue string, modRev, ver int64) model.WatchEvent {
	return model.WatchEvent{
		PersistedEvent: deletePersistedEvent(key, rev),
		PrevValue: &model.ValueRevision{
			Value:       model.ToValueOrHash(prevValue),
			ModRevision: modRev,
			Version:     ver,
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

func putRequest(key, value string) model.EtcdRequest {
	return model.EtcdRequest{
		Type:        model.Txn,
		LeaseGrant:  nil,
		LeaseRevoke: nil,
		Range:       nil,
		Txn: &model.TxnRequest{
			Conditions: nil,
			OperationsOnSuccess: []model.EtcdOperation{
				{
					Type: model.PutOperation,
					Put: model.PutOptions{
						Key:   key,
						Value: model.ToValueOrHash(value),
					},
				},
			},
			OperationsOnFailure: nil,
		},
		Defragment: nil,
	}
}

func putRequestWithLease(key, value string, leaseID int64) model.EtcdRequest {
	req := putRequest(key, value)
	req.Txn.OperationsOnSuccess[0].Put.LeaseID = leaseID
	return req
}

func deleteRequest(key string) model.EtcdRequest {
	return model.EtcdRequest{
		Type:        model.Txn,
		LeaseGrant:  nil,
		LeaseRevoke: nil,
		Range:       nil,
		Txn: &model.TxnRequest{
			Conditions: nil,
			OperationsOnSuccess: []model.EtcdOperation{
				{
					Type: model.DeleteOperation,
					Delete: model.DeleteOptions{
						Key: key,
					},
				},
			},
			OperationsOnFailure: nil,
		},
		Defragment: nil,
	}
}
