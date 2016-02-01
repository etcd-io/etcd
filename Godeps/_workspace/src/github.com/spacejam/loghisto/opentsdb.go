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

type openTSDBStat struct {
	Metric string
	Time   int64
	Value  float64
	Tags   map[string]string
}

type openTSDBStatArray []*openTSDBStat

func mapToTSDProtocolTags(tagMap map[string]string) string {
	tags := make([]string, 0, len(tagMap))
	for tag, value := range tagMap {
		tags = append(tags, fmt.Sprintf("%s=%s", tag, value))
	}
	return strings.Join(tags, " ")
}

func (stats openTSDBStatArray) ToRequest() []byte {
	var request bytes.Buffer
	for _, stat := range stats {
		request.Write([]byte(fmt.Sprintf("put %s %d %f %s\n",
			stat.Metric,
			stat.Time,
			stat.Value,
			mapToTSDProtocolTags(stat.Tags))))
	}
	return []byte(request.String())
}

func (metricSet *ProcessedMetricSet) toopenTSDBStats() openTSDBStatArray {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	stats := make([]*openTSDBStat, 0, len(metricSet.Metrics))
	i := 0
	for metric, value := range metricSet.Metrics {
		var tags = map[string]string{
			"host": hostname,
		}
		//TODO(tyler) custom tags
		stats = append(stats, &openTSDBStat{
			Metric: metric,
			Time:   metricSet.Time.Unix(),
			Value:  value,
			Tags:   tags,
		})
		i++
	}
	return stats
}

// OpenTSDBProtocol generates a wire representation of a ProcessedMetricSet
// for submission to an OpenTSDB instance.
func OpenTSDBProtocol(ms *ProcessedMetricSet) []byte {
	return ms.toopenTSDBStats().ToRequest()
}
