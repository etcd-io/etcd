// Copyright 2025 The etcd Authors
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

package report

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func TestPersistedRequests(t *testing.T) {
	lg := zaptest.NewLogger(t)

	tcs := []struct {
		name           string
		dataDirs       []string
		readerFunc     func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error)
		expectErr      string
		expectRequests []model.EtcdRequest
	}{
		{
			name:      "Error when empty data dir",
			dataDirs:  []string{},
			expectErr: "no data dirs",
		},
		{
			name:     "Success when no entries",
			dataDirs: []string{"etcd0"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				return []model.EtcdRequest{}, nil
			},
			expectRequests: []model.EtcdRequest{},
		},
		{
			name:     "Error when error on single node cluster",
			dataDirs: []string{"etcd0"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				return []model.EtcdRequest{}, errors.New("error reading")
			},
			expectErr: "error reading",
		},
		{
			name:     "Success when one member cluster",
			dataDirs: []string{"etcd0"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				return []model.EtcdRequest{
					{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
					{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
					{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
				}, nil
			},
			expectRequests: []model.EtcdRequest{
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
			},
		},
		{
			name:     "Success when three members agree on entries",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				return []model.EtcdRequest{
					{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
					{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
					{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
				}, nil
			},
			expectRequests: []model.EtcdRequest{
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
			},
		},
		{
			name:     "Success when three member have no entries",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				return []model.EtcdRequest{}, nil
			},
			expectRequests: []model.EtcdRequest{},
		},
		{
			name:     "Success when one member returned error in three node cluster",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd1":
					return []model.EtcdRequest{}, errors.New("error reading")
				default:
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				}
			},
			expectRequests: []model.EtcdRequest{
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
			},
		},
		{
			name:     "Success when one member returned empty in three node cluster",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd1":
					return []model.EtcdRequest{}, nil
				default:
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				}
			},
			expectRequests: []model.EtcdRequest{
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
			},
		},
		{
			name:     "Error when two members returned error in three node cluster",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd1", "etcd2":
					return []model.EtcdRequest{}, errors.New("error reading")
				default:
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				}
			},
			expectErr: "error reading",
		},
		{
			name:     "Success if members didn't observe whole history",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd0":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				case "etcd1":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
					}, nil
				case "etcd2":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
					}, nil
				default:
					panic("unexpected")
				}
			},
			expectRequests: []model.EtcdRequest{
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
			},
		},
		{
			name:     "Success if only one member observed history",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd0":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				case "etcd1", "etcd2":
					return []model.EtcdRequest{}, nil
				default:
					panic("unexpected")
				}
			},
			expectRequests: []model.EtcdRequest{
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
			},
		},
		{
			name:     "Success when one member observed different last entry",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd0":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				case "etcd1":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				case "etcd2":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 4}},
					}, nil
				default:
					panic("unexpected")
				}
			},
			expectRequests: []model.EtcdRequest{
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
				{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
			},
		},
		{
			name:     "Error when one member didn't observe whole history and others observed different last entry",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd0":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
					}, nil
				case "etcd1":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				case "etcd2":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 4}},
					}, nil
				default:
					panic("unexpected")
				}
			},
			expectErr: "unexpected differences between wal entries",
		},
		{
			name:     "Error when three members observed different last entry",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd0":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				case "etcd1":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 4}},
					}, nil
				case "etcd2":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 5}},
					}, nil
				default:
					panic("unexpected")
				}
			},
			expectErr: "unexpected differences between wal entries",
		},
		{
			name:     "Error when one member returned error and others differ on last entry",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd0":
					return []model.EtcdRequest{}, errors.New("error reading")
				case "etcd1":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				case "etcd2":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 4}},
					}, nil
				default:
					panic("unexpected")
				}
			},
			expectErr: "unexpected differences between wal entries",
		},
		{
			name:     "Error when one member observed empty history and others differ on last entry",
			dataDirs: []string{"etcd0", "etcd1", "etcd2"},
			readerFunc: func(lg *zap.Logger, dataDir string) ([]model.EtcdRequest, error) {
				switch dataDir {
				case "etcd0":
					return []model.EtcdRequest{}, nil
				case "etcd1":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 3}},
					}, nil
				case "etcd2":
					return []model.EtcdRequest{
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 1}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 2}},
						{Type: model.Compact, Compact: &model.CompactRequest{Revision: 4}},
					}, nil
				default:
					panic("unexpected")
				}
			},
			expectErr: "unexpected differences between wal entries",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			requests, err := persistedRequests(lg, tc.dataDirs, tc.readerFunc)
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectErr)
			}
			require.Equal(t, tc.expectRequests, requests)
		})
	}
}
