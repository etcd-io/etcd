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

// customRule1 Defines the custom rule
type customRule1 struct {
	// RefreshRate refresh rate in milliseconds
	RefreshRate int
	// RuleStatus the status of the rule
	RuleStatus bool
}

// NewCustomRule1 prepares custom rule 1
func NewCustomRule1(s *EtcdServer) RateLimiterRule {
	// By default the rule set is disabled, will automatically kick in once a rule is activated in the go routine
	cr1 := &customRule1{
		RefreshRate: 150000,
		RuleStatus:  false,
	}
	return cr1
}

// RefreshRateInMs refresh rate in milliseconds for custom rule 1
func (cr1 *customRule1) RefreshRateInMs() int {
	return cr1.RefreshRate
}

// Active defines the rule and returns an bool
func (cr1 *customRule1) Active() bool {
	return cr1.RuleStatus
}
