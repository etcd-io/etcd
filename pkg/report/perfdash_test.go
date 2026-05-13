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

package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWritePerfDashReportIncludesP999(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ARTIFACTS", tmpDir)

	r := newReport("%f", "put", true)
	r.stats.Lats = make([]float64, 1000)
	for i := range r.stats.Lats {
		r.stats.Lats[i] = float64(i) / 1000
	}

	r.writePerfDashReport("put")

	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	payload, err := os.ReadFile(filepath.Join(tmpDir, entries[0].Name()))
	require.NoError(t, err)

	var report perfdashFormattedReport
	require.NoError(t, json.Unmarshal(payload, &report))
	require.Len(t, report.DataItems, 1)
	require.Equal(t, 500.0, report.DataItems[0].Data.Perc50)
	require.Equal(t, 900.0, report.DataItems[0].Data.Perc90)
	require.Equal(t, 990.0, report.DataItems[0].Data.Perc99)
	require.Equal(t, 999.0, report.DataItems[0].Data.Perc999)
}
