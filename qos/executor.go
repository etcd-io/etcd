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
	"errors"
	"fmt"
	"strings"

	"github.com/antonmedv/expr"
	"go.etcd.io/etcd/qos/qospb"
	"go.uber.org/zap"
)

const (
	RuleExecutorExpr         = "exprExecutor"
	RuleExecutorCmpThreshold = "cmpThresholdExecutor"
	RuleExecutorGRPCMethod   = "gRPCMethodExecutor"
)

var (
	ErrInvalidRuleExecutorName = errors.New("qos: rule executor name is invalid")
)

type RuleExecutor interface {
	Run(r *qospb.QoSRule, Env map[string]interface{}) (bool, error)

	Name() string
}

func NewRuleExecutor(lg *zap.Logger, ruleType QoSRuleType) (RuleExecutor, error) {
	switch ruleType {
	case RuleTypeGRPCMethod:
		return NewGRPCMethodRuleExecutor(lg, RuleExecutorGRPCMethod), nil
	case RuleTypeSlowQuery, RuleTypeTraffic:
		return NewCompareRuleExecutor(lg, RuleExecutorCmpThreshold), nil
	case RuleTypeCustom:
		return NewExprRuleExecutor(lg, RuleExecutorExpr), nil
	default:
		return nil, ErrInvalidRuleExecutorName
	}
}

type exprRuleExecutor struct {
	lg   *zap.Logger
	name string
}

func NewExprRuleExecutor(lg *zap.Logger, name string) RuleExecutor {
	return &exprRuleExecutor{lg: lg, name: name}
}

func (ee *exprRuleExecutor) Run(r *qospb.QoSRule, env map[string]interface{}) (bool, error) {
	code, err := expr.Compile(r.Condition, expr.Env(env))
	if err != nil {
		ee.lg.Warn("failed to compile code", zap.String("condition", r.Condition), zap.Error(err))
		return false, err
	}
	result, err := expr.Run(code, env)
	if err != nil {
		ee.lg.Warn("failed to run expr code", zap.String("code", r.Condition), zap.Error(err))
		return false, err
	}
	return result.(bool), nil
}

func (ee *exprRuleExecutor) Name() string {
	return ee.name
}

type gRPCMethodRuleExecutor struct {
	lg   *zap.Logger
	name string
}

func NewGRPCMethodRuleExecutor(lg *zap.Logger, name string) RuleExecutor {
	return &gRPCMethodRuleExecutor{lg: lg, name: name}
}

func (ee *gRPCMethodRuleExecutor) Run(r *qospb.QoSRule, env map[string]interface{}) (bool, error) {
	gRPCMethod := env["gRPCMethod"].(string)
	key, ok := env["key"].(string)
	if !ok {
		return false, ErrEnvLackOfKey
	}
	if gRPCMethod == r.Subject.Name {
		if r.Subject.Prefix != "" && key != "" && !strings.HasPrefix(key, r.Subject.Prefix) {
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

func (ee *gRPCMethodRuleExecutor) Name() string {
	return ee.name
}

type cmpThresholdRuleExecutor struct {
	lg   *zap.Logger
	name string
}

func NewCompareRuleExecutor(lg *zap.Logger, name string) RuleExecutor {
	return &cmpThresholdRuleExecutor{lg: lg, name: name}
}

func (ce *cmpThresholdRuleExecutor) Run(r *qospb.QoSRule, env map[string]interface{}) (bool, error) {
	key, ok := env["key"].(string)
	if !ok {
		return false, ErrEnvLackOfKey
	}
	if r.Subject.Prefix != "" && !strings.HasPrefix(key, r.Subject.Prefix) {
		return false, nil
	}
	value, ok := env[r.Subject.Name].(int)
	if !ok {
		return false, fmt.Errorf("failed to get subject name %s", r.Subject.Name)
	}
	if uint64(value) > r.Threshold {
		return true, nil
	}
	return false, nil
}

func (ce *cmpThresholdRuleExecutor) Name() string {
	return ce.name
}
