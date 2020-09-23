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
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"strings"
)

// RateLimiter defines a rate limiter
type RateLimiter interface {
	// Limit decides whether to limit the current request based on the available quota.
	Limit(reqType string) bool
}

// allowAllRequests allows all the requests without filtering
type allowAllRequests struct{}

// Limit does not limit any request in this interface
func (*allowAllRequests) Limit(string) bool { return false }

const (
	etcdserverpbkv = "/etcdserverpb.KV/"
	kvPut          = etcdserverpbkv + "Put"
	kvRange        = etcdserverpbkv + "Range"
	kvDelete       = etcdserverpbkv + "Delete"
	kvCompact      = etcdserverpbkv + "Compact"
)

// rateLimiter rate limiter object
type rateLimiter struct {
	//s Etcd Server
	s *EtcdServer
	// maxRequestsPerSecond the max number of allowed requests/second
	maxRequestsPerSecond float64
	// ruleSet rate limiter ruleset
	ruleSet RateLimiterRuleSet
	// rateLimiterRequestTypeFilter rate limiter request Type filter
	rateLimiterRequestTypeFilter map[string]bool
	// rateLimiter rate checker to rate limit
	rateLimiter *rate.Limiter
}

// NewRateLimiter returns a new rateLimiter
func NewRateLimiter(s *EtcdServer, name string) RateLimiter {
	lg := s.getLogger()

	// rateLimiterRequestTypeFilter Accept from the user as input
	// compare with this, flip bool values according user input
	// below configuration represents default prefs which are subject to rate limiting,
	// type of requests (key) : filter enabled for these kinds of request (value)
	var rateLimiterRequestTypeFilter = map[string]bool{
		kvPut:     false,
		kvRange:   false,
		kvDelete:  false,
		kvCompact: false,
	}

	// Check if any request filters were applied
	if len(s.Cfg.RateLimiterRequestFilter) > 0 {
		// Split the user input into each type, for example: Put, Range, Delete, etc
		requestTypes := strings.Split(s.Cfg.RateLimiterRequestFilter, ",")
		for _, requestType := range requestTypes {
			// Check if the "/etcdserverpb.KV/"  requestType exists in our map.
			current := etcdserverpbkv + strings.Title(strings.ToLower(strings.TrimSpace(requestType)))
			if _, ok := rateLimiterRequestTypeFilter[current]; ok {
				// if true, enable rate limiting for that type
				rateLimiterRequestTypeFilter[current] = true
			}
		}
	}

	// Check if rate limiter is enabled by checking if value is greater than 0
	if s.Cfg.RequestsPerSecondLimit > 0 {
		// Check if any rule flags were passed, else enable a simple rate limiter
		if len(strings.TrimSpace(s.Cfg.EnableRateLimiter)) <= 0 {
			if lg != nil {
				lg.Info(
					"simple rate limiter will be enabled",
					zap.String("rate-limiter-name", name),
					zap.Float64("rate-limit", s.Cfg.RequestsPerSecondLimit),
				)
			}
			return &rateLimiter{
				s,
				s.Cfg.RequestsPerSecondLimit,
				nil,
				rateLimiterRequestTypeFilter,
				rate.NewLimiter(rate.Limit(s.Cfg.RequestsPerSecondLimit), int(s.Cfg.RequestsPerSecondLimit)),
			}
		}
		// Initialize the rate limiter, custom rule set and return
		quotaLogOnce.Do(func() {
			if lg != nil {
				lg.Info(
					"custom rate limiter will be enabled",
					zap.String("rate-limiter-name", name),
					zap.Float64("rate-limit", s.Cfg.RequestsPerSecondLimit),
				)
			}
		})
		return &rateLimiter{
			s,
			s.Cfg.RequestsPerSecondLimit,
			NewCustomRuleSet(s),
			rateLimiterRequestTypeFilter,
			rate.NewLimiter(rate.Limit(s.Cfg.RequestsPerSecondLimit), int(s.Cfg.RequestsPerSecondLimit)),
		}
	}

	// No rate limiters were enabled, will let each request pass through transparently.
	return &allowAllRequests{}
}

// Limit decides whether the server is allowed to serve request based on defined rules
func (r rateLimiter) Limit(reqType string) bool {
	// Check if this type of request is to be filtered at all, if not, skip
	if r.rateLimiterRequestTypeFilter[reqType] {
		// If, ruleset is not available and status does not have any rules enabled, let request
		// pass through
		// Else, rate limit (which means, either the simple rate limiter was triggered or a rule was
		// enabled
		if r.ruleSet != nil && !r.ruleSet.Status() {
			// Allow means we're not rate limiting, so flip the condition
			// and allow everything that passes through
			return false
		}
		return !r.rateLimiter.Allow()
	}
	// If disabled or not filtered request type, let the request pass through because we're NOT
	// rate limiting
	return false
}
