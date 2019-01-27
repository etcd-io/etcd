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

package mock

import (
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/store/tikv/oracle"
)

// Store implements kv.Storage interface.
type Store struct {
	Client kv.Client
}

// GetClient implements kv.Storage interface.
func (s *Store) GetClient() kv.Client { return s.Client }

// GetOracle implements kv.Storage interface.
func (s *Store) GetOracle() oracle.Oracle { return nil }

// Begin implements kv.Storage interface.
func (s *Store) Begin() (kv.Transaction, error) { return nil, nil }

// BeginWithStartTS implements kv.Storage interface.
func (s *Store) BeginWithStartTS(startTS uint64) (kv.Transaction, error) { return s.Begin() }

// GetSnapshot implements kv.Storage interface.
func (s *Store) GetSnapshot(ver kv.Version) (kv.Snapshot, error) { return nil, nil }

// Close implements kv.Storage interface.
func (s *Store) Close() error { return nil }

// UUID implements kv.Storage interface.
func (s *Store) UUID() string { return "mock" }

// CurrentVersion implements kv.Storage interface.
func (s *Store) CurrentVersion() (kv.Version, error) { return kv.Version{}, nil }

// SupportDeleteRange implements kv.Storage interface.
func (s *Store) SupportDeleteRange() bool { return false }
