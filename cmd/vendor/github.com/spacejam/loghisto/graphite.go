// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.  See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.
//
// Author: Tyler Neely (t@jujit.su)

package loghisto

import (
	"bytes"
	"fmt"
	"os"
	"strings"

)

type graphiteStat struct {
	Metric string
	Time   int64
	Value  float64
	Host   string
}

type graphiteStatArray []*graphiteStat

func (stats graphiteStatArray) ToRequest() []byte {
	var request bytes.Buffer
	for _, stat := range stats {
		request.Write([]byte(fmt.Sprintf("cockroach.%s.%s %f %d\n",
			stat.Host,
			strings.Replace(stat.Metric, "_", ".", -1),
			stat.Value,
			stat.Time,
		)))
	}
	return []byte(request.String())
}

func (metricSet *ProcessedMetricSet) tographiteStats() graphiteStatArray {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	stats := make([]*graphiteStat, 0, len(metricSet.Metrics))
	i := 0
	for metric, value := range metricSet.Metrics {
		//TODO(tyler) custom tags
		stats = append(stats, &graphiteStat{
			Metric: metric,
			Time:   metricSet.Time.Unix(),
			Value:  value,
			Host:   hostname,
		})
		i++
	}
	return stats
}

// GraphiteProtocol generates a wire representation of a ProcessedMetricSet
// for submission to a Graphite Carbon instance using the plaintext protocol.
func GraphiteProtocol(ms *ProcessedMetricSet) []byte {
	return ms.tographiteStats().ToRequest()
}
