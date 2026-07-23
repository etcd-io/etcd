// Copyright 2026 The etcd Authors
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

package v3compactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestAutoPeriodicShouldCompact exercises the projection decision that drives the
// mode, using values in the spirit of the production defaults (high-water 0.7,
// lead time 5m). The key property: a bigger growth rate makes it fire earlier
// (at a smaller current size), giving the reclaim more headroom.
func TestAutoPeriodicShouldCompact(t *testing.T) {
	defer func(hw float64, lt time.Duration) {
		AutoPeriodicHighWater, AutoPeriodicLeadTime = hw, lt
	}(AutoPeriodicHighWater, AutoPeriodicLeadTime)
	AutoPeriodicHighWater = 0.7
	AutoPeriodicLeadTime = 5 * time.Minute

	const quota = 1000 // high-water = 700
	ac := &AutoPeriodic{maxBytes: quota}

	tests := []struct {
		name       string
		size       int64
		ratePerSec float64
		want       bool
	}{
		{"idle, well below", 100, 0, false},
		{"idle, just below high-water", 699, 0, false},
		{"idle, at high-water", 700, 0, true},
		// At 1 byte/sec, 5m projects +300B: fires 300 below the high-water.
		{"slow growth, not yet", 399, 1, false},
		{"slow growth, projected to cross", 401, 1, true},
		// A fast writer (10 B/s -> +3000B projected) fires far earlier, at only
		// ~10% of quota, leaving maximal headroom for the reclaim.
		{"fast growth fires early", 100, 10, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ac.shouldCompact(tc.size, tc.ratePerSec))
		})
	}
}

// TestAutoPeriodicDefaults sanity-checks the production defaults: a conservative
// backstop with generous headroom and a lead time that comfortably covers a
// compaction+reclaim.
func TestAutoPeriodicDefaults(t *testing.T) {
	assert.InDelta(t, 0.7, AutoPeriodicHighWater, 0.001)
	assert.LessOrEqual(t, AutoPeriodicSampleInterval, 30*time.Second)
	assert.GreaterOrEqual(t, AutoPeriodicLeadTime, time.Minute)
	assert.GreaterOrEqual(t, AutoPeriodicCooldown, 30*time.Second)
	assert.Positive(t, autoPeriodicRateAlpha)
	assert.Less(t, autoPeriodicRateAlpha, 1.0)
}
