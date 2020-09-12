// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "qs IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package qos

import (
	"strings"
	"sync"

	"go.etcd.io/etcd/v3/qos/qospb"
	"go.uber.org/zap"
)

type QoSRuleType string

const (
	RuleTypeGRPCMethod QoSRuleType = "gRPCMethod"
	RuleTypeSlowQuery  QoSRuleType = "slowquery"
	RuleTypeTraffic    QoSRuleType = "traffic"
	RuleTypeCustom     QoSRuleType = "custom"

	RulePriorityMax = 9
	RulePriorityMin = 0
)

type RequestContext struct {
	GRPCMethod      string
	Key             string
	RangeKeyNum     int
	DBUsedByte      int
	MatchedRuleName string
}

type Enforcer interface {
	SyncRule(r *qospb.QoSRule) error

	DeleteRule(r *qospb.QoSRule) error

	GetToken(r *RequestContext) bool

	PutToken(r *RequestContext) bool
}

type QosEnforcer struct {
	mu                sync.Mutex
	lg                *zap.Logger
	objectRateLimiter map[string]RateLimiter
	rulePriList       []map[string]*qospb.QoSRule
}

func NewQoSEnforcer(lg *zap.Logger) Enforcer {
	return &QosEnforcer{
		lg:                lg,
		objectRateLimiter: make(map[string]RateLimiter),
		rulePriList:       make([]map[string]*qospb.QoSRule, RulePriorityMax+1),
	}
}

func (qe *QosEnforcer) SyncRule(r *qospb.QoSRule) error {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	if r.Priority < RulePriorityMin || r.Priority > RulePriorityMax {
		return ErrInvalidPriorityRule
	}
	limiter, err := NewRateLimiter(r.Ratelimiter, r.Qps)
	if err != nil {
		qe.lg.Error("failed to add qos rule", zap.Error(err))
		return err
	}
	qe.lg.Debug("create qos rule ratelimiter", zap.Any("rule", *r))
	qe.objectRateLimiter[r.RuleName] = limiter
	if qe.rulePriList[r.Priority] != nil {
		qe.rulePriList[r.Priority][r.RuleName] = r
	} else {
		set := make(map[string]*qospb.QoSRule)
		set[r.RuleName] = r
		qe.rulePriList[r.Priority] = set
	}
	return nil
}

func (qe *QosEnforcer) DeleteRule(r *qospb.QoSRule) error {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	if r.Priority < RulePriorityMin || r.Priority > RulePriorityMax {
		return ErrInvalidPriorityRule
	}
	delete(qe.objectRateLimiter, r.RuleName)
	set := qe.rulePriList[r.Priority]
	if set != nil {
		delete(set, r.RuleName)
	}
	return nil
}

func (qe *QosEnforcer) GetToken(r *RequestContext) bool {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	// to do, optimize performance, used lru cache to store matched rule
	rule, err := qe.matchRule(r)
	if err != nil {
		return true
	}
	qe.lg.Info("matched rule", zap.Any("object", r), zap.Any("rule", rule))
	limiter, ok := qe.objectRateLimiter[rule.RuleName]
	if ok {
		return limiter.GetToken()
	}
	return true
}

func (qe *QosEnforcer) PutToken(r *RequestContext) bool {
	qe.mu.Lock()
	defer qe.mu.Unlock()
	limiter, ok := qe.objectRateLimiter[r.MatchedRuleName]
	if ok {
		return limiter.PutToken()
	}
	return true
}

func (qe *QosEnforcer) matchRule(r *RequestContext) (*qospb.QoSRule, error) {
	matched := false
	qe.lg.Debug("start to match rule", zap.Any("object", r))
	// to do,optimize and benchmark performance
	for i := RulePriorityMax; i >= RulePriorityMin; i-- {
		for _, rule := range qe.rulePriList[i] {
			qe.lg.Debug("matching rule", zap.Any("rule", rule))
			executor, err := NewRuleExecutor(qe.lg, QoSRuleType(rule.RuleType))
			if err != nil {
				qe.lg.Warn("failed to create new rule executor", zap.Error(err), zap.Any("rule", rule))
				return nil, err
			}
			matched, err = executor.Run(rule, qe.populateEnv(r))
			if err != nil {
				qe.lg.Warn("failed to run rule executor", zap.Error(err), zap.Any("rule", rule))
				return nil, err
			}
			if matched {
				return rule, nil
			}
		}
	}
	return nil, ErrNoRuleMatched
}

func (qe *QosEnforcer) populateEnv(r *RequestContext) map[string]interface{} {
	env := make(map[string]interface{})
	env["gRPCMethod"] = r.GRPCMethod
	env["key"] = r.Key
	env["prefix"] = strings.HasPrefix
	env["RangeKeyNum"] = r.RangeKeyNum
	env["DBUsedByte"] = r.DBUsedByte
	return env
}
