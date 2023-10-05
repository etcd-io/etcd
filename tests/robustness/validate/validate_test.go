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

package validate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/tests/v3/framework/testutils"
	"go.etcd.io/etcd/tests/v3/robustness/model"
	"go.etcd.io/etcd/tests/v3/robustness/report"
)

func TestValidate(t *testing.T) {
	testdataPath := testutils.MustAbsPath("../testdata/")
	files, err := os.ReadDir(testdataPath)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 1)
	for _, file := range files {
		t.Run(file.Name(), func(t *testing.T) {
			path := filepath.Join(testdataPath, file.Name())
			reports, err := report.LoadClientReports(path)
			assert.NoError(t, err)
			visualize := ValidateAndReturnVisualize(t, zaptest.NewLogger(t), Config{}, reports)

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
		name    string
		reports []report.ClientReport
	}{
		{
			name: "earlier event after bookmark in separate request",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:      "a",
								Revision: 100,
							},
							Responses: []model.WatchResponse{
								{
									IsProgressNotify: true,
									Revision:         100,
								},
							},
						},
						{
							Request: model.WatchRequest{
								Key:      "a",
								Revision: 99,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										{
											Event: model.Event{
												Type: model.PutOperation,
												Key:  "a",
												Value: model.ValueOrHash{
													Value: "99",
												},
											},
											Revision: 99,
										},
									},
									Revision: 100,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "earlier event after in separate request",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:      "a",
								Revision: 100,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										{
											Event: model.Event{
												Type: model.PutOperation,
												Key:  "a",
												Value: model.ValueOrHash{
													Value: "100",
												},
											},
											Revision: 100,
										},
									},
									Revision: 100,
								},
							},
						},
						{
							Request: model.WatchRequest{
								Key:      "a",
								Revision: 99,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										{
											Event: model.Event{
												Type: model.PutOperation,
												Key:  "a",
												Value: model.ValueOrHash{
													Value: "99",
												},
											},
											Revision: 99,
										},
									},
									Revision: 100,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "duplicated event between two separate requests",
			reports: []report.ClientReport{
				{
					Watch: []model.WatchOperation{
						{
							Request: model.WatchRequest{
								Key:      "a",
								Revision: 100,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										{
											Event: model.Event{
												Type: model.PutOperation,
												Key:  "a",
												Value: model.ValueOrHash{
													Value: "100",
												},
											},
											Revision: 100,
										},
									},
									Revision: 100,
								},
							},
						},
						{
							Request: model.WatchRequest{
								Key:      "a",
								Revision: 100,
							},
							Responses: []model.WatchResponse{
								{
									Events: []model.WatchEvent{
										{
											Event: model.Event{
												Type: model.PutOperation,
												Key:  "a",
												Value: model.ValueOrHash{
													Value: "100",
												},
											},
											Revision: 100,
										},
									},
									Revision: 100,
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			validateWatch(t, Config{ExpectRevisionUnique: true}, tc.reports)
		})
	}
}
