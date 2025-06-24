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
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

type Metrics struct {
	Perc50 float64 `json:"Perc50"`
	Perc90 float64 `json:"Perc90"`
	Perc99 float64 `json:"Perc99"`
}

type Labels struct {
	Metric string `json:"Metric"`
}

type DataItem struct {
	Data   Metrics `json:"data"`
	Labels Labels  `json:"labels"`
	Unit   string  `json:"unit"`
}

type perfdashFormattedReport struct {
	Version   string     `json:"version"`
	DataItems []DataItem `json:"dataItems"`
}

func (r *report) writePerfDashReport(reportName string) {
	pcls, data := Percentiles(r.stats.Lats)
	pclsData := make(map[float64]float64)
	for i := 0; i < len(pcls); i++ {
		pclsData[pcls[i]] = data[i] * 1000 // Since the reported data is in seconds, convert to ms.
	}
	report := perfdashFormattedReport{
		Version: "v1",
		DataItems: []DataItem{
			{
				Data: Metrics{
					Perc50: math.Round(pclsData[50]*10000) / 10000,
					Perc90: math.Round(pclsData[90]*10000) / 10000,
					Perc99: math.Round(pclsData[99]*10000) / 10000,
				},
				Unit: "ms",
				Labels: Labels{
					Metric: "APIResponsiveness",
				},
			},
		},
	}
	reportB, _ := json.MarshalIndent(report, "", "  ")

	artifactsDir := os.Getenv("ARTIFACTS")
	if artifactsDir == "" {
		artifactsDir = "./_artifacts"
	}

	fileName := fmt.Sprintf("etcd_perf_%s_%s.json", reportName, time.Now().UTC().Format(time.RFC3339))
	err := os.MkdirAll(artifactsDir, 0o755)
	if err != nil {
		fmt.Println("Error creating artifacts directory:", err)
	}
	destPath := filepath.Join(artifactsDir, fileName)
	err = os.WriteFile(destPath, reportB, 0o644)
	if err != nil {
		fmt.Println("Error writing to file:", err)
	}
	fmt.Println("Successfully created a JSON perf report at", destPath)
}
