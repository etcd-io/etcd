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

import (
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

// RateLimiterRuleSet rate limiter rule set
type RateLimiterRuleSet interface {
	// Compute Computes the rules
	Compute()
	// Status Checks for any rule violations
	Status() bool
}

// customRuleSet custom rule set
type customRuleSet struct {
	// RuleStatus an atomic bool value is updated at rule updates
	RuleStatus atomic.Bool
	// RuleStatusMap a map of statuses of all rules
	RuleStatusMap *sync.Map
	// terminationChannel terminates all go routines when server is closed
	terminationChannel chan bool
	// updateRuleChannel to communicate rule updates
	updateRuleChannel chan struct{}
	// CustomRules map of rules to the rate limiter rule
	CustomRules map[string]RateLimiterRule
	// server Etcd server reference
	server *EtcdServer
}

// NewCustomRuleSet prepares a custom rule set
func NewCustomRuleSet(s *EtcdServer) RateLimiterRuleSet {
	// Check for any custom maps already defined
	customRuleMap := s.Cfg.CustomRuleMap
	if customRuleMap == nil {
		customRuleMap = InitiateRules(s)
	}

	// By default the rule set is disabled,
	// will automatically kick in once a rule is activated in the go routine
	crs := &customRuleSet{
		RuleStatusMap:      &sync.Map{},
		CustomRules:        customRuleMap,
		terminationChannel: s.RulesetRoutine,
		server:             s,
		updateRuleChannel:  make(chan struct{}),
	}

	// Initiate all compute routines, if any
	crs.Compute()
	return crs
}

// InitiateRules initiates all the rules as per user configuration
func InitiateRules(s *EtcdServer) map[string]RateLimiterRule {
	lg := s.lg
	// Build a map of all the rules passed and a map of all rules initiated
	enabledRuleset := map[string]bool{}
	customRules := map[string]RateLimiterRule{}

	// Check for all the rules user has enabled and prepare the map
	for _, currentRule := range strings.Split(s.Cfg.EnableRateLimiter, ",") {
		enabledRuleset[strings.ToLower(strings.TrimSpace(currentRule))] = true
	}

	// Go through each rule definition and check what was enabled
	// Define rule1
	ruleName1 := "rule1"
	if _, ok := enabledRuleset[ruleName1]; ok {
		customRules[ruleName1] = NewCustomRule1(s)
		if lg != nil {
			lg.Info("custom rule enabled", zap.String("rule-name", ruleName1))
		}
	}

	// Define rule2
	ruleName2 := "rule2"
	if _, ok := enabledRuleset[ruleName2]; ok {
		customRules[ruleName2] = NewCustomRule2(s)
		if lg != nil {
			lg.Info("custom rule enabled", zap.String("rule-name", ruleName2))
		}
	}
	return customRules
}

// Compute rule checker compute
func (crs *customRuleSet) Compute() {
	lg := crs.server.lg
	// Check if no rules are enabled, if so no need to actually run a go routine.
	if len(crs.CustomRules) == 0 {
		if lg != nil {
			lg.Info("none of the custom rules were available")
		}
		return
	}

	// Go through each active rule and initiate the go routine
	for ruleName := range crs.CustomRules {
		go crs.ComputeRoutine(ruleName)
	}

	// Spin up a go routine to compute status at the frequency defined by rule
	go crs.StatusRoutine()
}

// ComputeRoutine is running the go routine
func (crs *customRuleSet) ComputeRoutine(ruleName string) {
	// Initiate a ticker to refresh the rule periodically
	ticker := time.NewTicker(time.Duration(crs.CustomRules[ruleName].RefreshRateInMs()) * time.Millisecond)
	previousValue := false

	// Run forever
	for {
		select {
		case <-ticker.C:
			// Load the current value of the rule
			// If value is same as previous value, move on
			// Else, update the value and channel a change
			// The change will trigger the statusMap to be recomputed
			currentValue := crs.CustomRules[ruleName].Active()
			if currentValue != previousValue {
				crs.RuleStatusMap.Store(ruleName, currentValue)
				previousValue = currentValue
				crs.updateRuleChannel <- struct{}{}
			}
		case <-crs.terminationChannel:
			ticker.Stop()
			return
		}
	}
}

// Status check current status of rate limiter
func (crs *customRuleSet) Status() bool {
	return crs.RuleStatus.Load()
}

// StatusRoutine compute the status of the rule every ticker time
func (crs *customRuleSet) StatusRoutine() {
	for {
		select {
		case <-crs.updateRuleChannel:
			// Initialise the current result false
			// Presumes none of the rules are active
			var currentResult bool
			// Go through each of the rule statuses and check if any are active
			crs.RuleStatusMap.Range(func(key, value interface{}) bool {
				current := value.(bool)
				if current {
					// Found an active one, update current result
					currentResult = true
				}
				return true
			})
			// Update the RuleStatus accordingly
			crs.RuleStatus.Store(currentResult)
		case <-crs.terminationChannel:
			return
		}
	}
}
