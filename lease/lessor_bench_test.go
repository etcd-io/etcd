// Copyright 2018 The etcd Authors
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

package lease

import (
	"os"
	"testing"

	"go.etcd.io/etcd/mvcc/backend"
	"go.uber.org/zap"
)

func BenchmarkLessorFindExpired1(b *testing.B)       { benchmarkLessorFindExpired(1, b) }
func BenchmarkLessorFindExpired10(b *testing.B)      { benchmarkLessorFindExpired(10, b) }
func BenchmarkLessorFindExpired100(b *testing.B)     { benchmarkLessorFindExpired(100, b) }
func BenchmarkLessorFindExpired1000(b *testing.B)    { benchmarkLessorFindExpired(1000, b) }
func BenchmarkLessorFindExpired10000(b *testing.B)   { benchmarkLessorFindExpired(10000, b) }
func BenchmarkLessorFindExpired100000(b *testing.B)  { benchmarkLessorFindExpired(100000, b) }
func BenchmarkLessorFindExpired1000000(b *testing.B) { benchmarkLessorFindExpired(1000000, b) }

func BenchmarkLessorGrant1(b *testing.B)       { benchmarkLessorGrant(1, b) }
func BenchmarkLessorGrant10(b *testing.B)      { benchmarkLessorGrant(10, b) }
func BenchmarkLessorGrant100(b *testing.B)     { benchmarkLessorGrant(100, b) }
func BenchmarkLessorGrant1000(b *testing.B)    { benchmarkLessorGrant(1000, b) }
func BenchmarkLessorGrant10000(b *testing.B)   { benchmarkLessorGrant(10000, b) }
func BenchmarkLessorGrant100000(b *testing.B)  { benchmarkLessorGrant(100000, b) }
func BenchmarkLessorGrant1000000(b *testing.B) { benchmarkLessorGrant(1000000, b) }

func BenchmarkLessorRenew1(b *testing.B)       { benchmarkLessorRenew(1, b) }
func BenchmarkLessorRenew10(b *testing.B)      { benchmarkLessorRenew(10, b) }
func BenchmarkLessorRenew100(b *testing.B)     { benchmarkLessorRenew(100, b) }
func BenchmarkLessorRenew1000(b *testing.B)    { benchmarkLessorRenew(1000, b) }
func BenchmarkLessorRenew10000(b *testing.B)   { benchmarkLessorRenew(10000, b) }
func BenchmarkLessorRenew100000(b *testing.B)  { benchmarkLessorRenew(100000, b) }
func BenchmarkLessorRenew1000000(b *testing.B) { benchmarkLessorRenew(1000000, b) }

func BenchmarkLessorRevoke1(b *testing.B)       { benchmarkLessorRevoke(1, b) }
func BenchmarkLessorRevoke10(b *testing.B)      { benchmarkLessorRevoke(10, b) }
func BenchmarkLessorRevoke100(b *testing.B)     { benchmarkLessorRevoke(100, b) }
func BenchmarkLessorRevoke1000(b *testing.B)    { benchmarkLessorRevoke(1000, b) }
func BenchmarkLessorRevoke10000(b *testing.B)   { benchmarkLessorRevoke(10000, b) }
func BenchmarkLessorRevoke100000(b *testing.B)  { benchmarkLessorRevoke(100000, b) }
func BenchmarkLessorRevoke1000000(b *testing.B) { benchmarkLessorRevoke(1000000, b) }

func benchmarkLessorFindExpired(size int, b *testing.B) {
	lg := zap.NewNop()
	be, tmpPath := backend.NewDefaultTmpBackend()
	le := newLessor(lg, be, LessorConfig{MinLeaseTTL: minLeaseTTL})
	defer le.Stop()
	defer cleanup(be, tmpPath)
	le.Promote(0)
	for i := 0; i < size; i++ {
		le.Grant(LeaseID(i), int64(100+i))
	}
	le.mu.Lock() //Stop the findExpiredLeases call in the runloop
	defer le.mu.Unlock()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		le.findExpiredLeases(1000)
	}
}

func benchmarkLessorGrant(size int, b *testing.B) {
	lg := zap.NewNop()
	be, tmpPath := backend.NewDefaultTmpBackend()
	le := newLessor(lg, be, LessorConfig{MinLeaseTTL: minLeaseTTL})
	defer le.Stop()
	defer cleanup(be, tmpPath)
	for i := 0; i < size; i++ {
		le.Grant(LeaseID(i), int64(100+i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		le.Grant(LeaseID(i+size), int64(100+i+size))
	}
}

func benchmarkLessorRevoke(size int, b *testing.B) {
	lg := zap.NewNop()
	be, tmpPath := backend.NewDefaultTmpBackend()
	le := newLessor(lg, be, LessorConfig{MinLeaseTTL: minLeaseTTL})
	defer le.Stop()
	defer cleanup(be, tmpPath)
	for i := 0; i < size; i++ {
		le.Grant(LeaseID(i), int64(100+i))
	}
	for i := 0; i < b.N; i++ {
		le.Grant(LeaseID(i+size), int64(100+i+size))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		le.Revoke(LeaseID(i + size))
	}
}

func benchmarkLessorRenew(size int, b *testing.B) {
	lg := zap.NewNop()
	be, tmpPath := backend.NewDefaultTmpBackend()
	le := newLessor(lg, be, LessorConfig{MinLeaseTTL: minLeaseTTL})
	defer le.Stop()
	defer cleanup(be, tmpPath)
	for i := 0; i < size; i++ {
		le.Grant(LeaseID(i), int64(100+i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		le.Renew(LeaseID(i))
	}
}

func cleanup(b backend.Backend, path string) {
	b.Close()
	os.Remove(path)
}
