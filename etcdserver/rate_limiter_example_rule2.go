// Copyright 2020 The etcd Authors
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

package etcdserver

// customRule2 Defines the custom rule
type customRule2 struct {
	// RefreshRate refresh rate of rules in milliseconds
	RefreshRate int
	// RuleStatus the status of the rule
	RuleStatus bool
}

// NewCustomRule2 prepares custom rule 2
func NewCustomRule2(s *EtcdServer) RateLimiterRule {
	// By default the rule set is disabled, will automatically kick in once a rule is activated in the go routine
	cr2 := &customRule2{
		RefreshRate: 150000,
		RuleStatus:  true,
	}
	return cr2
}

// RefreshRateInMs refresh rate in milliseconds for custom rule 2
func (cr2 *customRule2) RefreshRateInMs() int {
	return cr2.RefreshRate
}

// Active defines the rule and returns an bool
func (cr2 *customRule2) Active() bool {
	return cr2.RuleStatus
}
