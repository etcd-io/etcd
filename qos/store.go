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
	"bytes"
	"errors"
	"sync"

	"go.etcd.io/etcd/etcdserver/cindex"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/mvcc/backend"
	"go.etcd.io/etcd/qos/qospb"
	"go.uber.org/zap"
)

var (
	enableFlagKey = []byte("qosEnabled")
	qosEnabled    = []byte{1}
	qosDisabled   = []byte{0}

	qosBucketName     = []byte("qos")
	qosRuleBucketName = []byte("qosRules")

	ErrQoSNotEnabled       = errors.New("qos: qos is not enabled")
	ErrQoSRuleAlreadyExist = errors.New("qos: QoSRule already exists")
	ErrQoSRuleEmpty        = errors.New("qos: QoSRule name is empty")
	ErrQoSRuleNotFound     = errors.New("qos: QoSRule not found")
	ErrQoSRateExceeded     = errors.New("qos: rate exceeded ")
	ErrInvalidPriorityRule = errors.New("qos: invalid rule priority")
	ErrNoRuleMatched       = errors.New("qos: no rules are matched")
	ErrEnvLackOfKey        = errors.New("qos: lack of env key")
)

// QoSStore defines qos storage interface.
type QoSStore interface {

	// Recover recovers the state of qos store from the given backend
	Recover(b backend.Backend)

	// QoSEnable turns on the qos feature
	QoSEnable(r *pb.QoSEnableRequest) error

	// QoSDisable turns on the qos feature
	QoSDisable(r *pb.QoSDisableRequest) error

	// IsQoSEnabled returns true if the qos feature is enabled.
	IsQoSEnabled() bool

	// QoSRuleAdd adds a new QoSRule into the backend store
	QoSRuleAdd(r *pb.QoSRuleAddRequest) (*pb.QoSRuleAddResponse, error)

	// QoSRuleDelete deletes a QoSRule from the backend store
	QoSRuleDelete(r *pb.QoSRuleDeleteRequest) (*pb.QoSRuleDeleteResponse, error)

	// QoSRuleUpdate updates a QoSRule into the backend store
	QoSRuleUpdate(r *pb.QoSRuleUpdateRequest) (*pb.QoSRuleUpdateResponse, error)

	// QoSRuleGet gets the detailed information of a QoSRules
	QoSRuleGet(r *pb.QoSRuleGetRequest) (*pb.QoSRuleGetResponse, error)

	// QoSRuleList list the detailed information of all QoSRules
	QoSRuleList(r *pb.QoSRuleListRequest) (*pb.QoSRuleListResponse, error)

	// GetToken returns true if rate do not exceed,otherwise return false
	GetToken(r *RequestContext) bool

	// PutToken returns token to rate limiter, return true if returns succeed,otherwise return false
	PutToken(r *RequestContext) bool

	// Close does cleanup of QoSStore
	Close() error
}

type qosStore struct {
	lg        *zap.Logger
	be        backend.Backend
	enabled   bool
	enabledMu sync.RWMutex
	ci        cindex.ConsistentIndexer
	enforcer  Enforcer
}

func (qs *qosStore) QoSEnable(r *pb.QoSEnableRequest) error {
	qs.enabledMu.Lock()
	defer qs.enabledMu.Unlock()
	if qs.enabled {
		qs.lg.Info("qos is already enabled; ignored qos enable request")
		return nil
	}
	b := qs.be
	tx := b.BatchTx()
	tx.Lock()
	defer func() {
		tx.Unlock()
		b.ForceCommit()
	}()

	tx.UnsafePut(qosBucketName, enableFlagKey, qosEnabled)

	qs.enabled = true
	qs.saveConsistentIndex(tx)

	qs.lg.Info("enabled qos")
	return nil
}

func (qs *qosStore) QoSDisable(r *pb.QoSDisableRequest) error {
	qs.enabledMu.Lock()
	defer qs.enabledMu.Unlock()
	if !qs.enabled {
		return ErrQoSNotEnabled
	}
	b := qs.be
	tx := b.BatchTx()
	tx.Lock()
	tx.UnsafePut(qosBucketName, enableFlagKey, qosDisabled)
	qs.saveConsistentIndex(tx)
	tx.Unlock()
	b.ForceCommit()

	qs.enabled = false
	qs.lg.Info("disabled qos")
	return nil
}

func (qs *qosStore) Close() error {
	qs.enabledMu.Lock()
	defer qs.enabledMu.Unlock()
	if !qs.enabled {
		return nil
	}
	return nil
}

func (qs *qosStore) Recover(be backend.Backend) {
	enabled := false
	qs.be = be
	tx := be.BatchTx()
	tx.Lock()
	_, vs := tx.UnsafeRange(qosBucketName, enableFlagKey, nil, 0)
	if len(vs) == 1 {
		if bytes.Equal(vs[0], qosEnabled) {
			enabled = true
		}
	}

	tx.Unlock()
	qs.enabledMu.Lock()
	qs.enabled = enabled
	qs.enabledMu.Unlock()
}

func (qs *qosStore) QoSRuleAdd(r *pb.QoSRuleAddRequest) (*pb.QoSRuleAddResponse, error) {
	tx := qs.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	QoSRule := getQoSRule(qs.lg, tx, r.QosRule.RuleName)
	if QoSRule != nil {
		return nil, ErrQoSRuleAlreadyExist
	}

	putQoSRule(qs.lg, tx, r.QosRule)
	qs.enforcer.SyncRule(r.QosRule)

	qs.saveConsistentIndex(tx)

	qs.lg.Info("added a QoSRule", zap.String("name", r.QosRule.RuleName))
	return &pb.QoSRuleAddResponse{}, nil
}

func (qs *qosStore) QoSRuleUpdate(r *pb.QoSRuleUpdateRequest) (*pb.QoSRuleUpdateResponse, error) {
	tx := qs.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	rule := getQoSRule(qs.lg, tx, r.QosRule.RuleName)
	if rule == nil {
		return nil, ErrQoSRuleNotFound
	}

	putQoSRule(qs.lg, tx, r.QosRule)
	qs.enforcer.SyncRule(r.QosRule)

	qs.saveConsistentIndex(tx)

	qs.lg.Info("updated a QoSRule", zap.String("name", r.QosRule.RuleName))
	return &pb.QoSRuleUpdateResponse{}, nil
}

func (qs *qosStore) QoSRuleDelete(r *pb.QoSRuleDeleteRequest) (*pb.QoSRuleDeleteResponse, error) {
	tx := qs.be.BatchTx()
	tx.Lock()
	defer tx.Unlock()

	rule := getQoSRule(qs.lg, tx, r.RuleName)
	if rule == nil {
		return nil, ErrQoSRuleNotFound
	}

	delQoSRule(tx, r.RuleName)

	qs.saveConsistentIndex(tx)
	qs.enforcer.DeleteRule(rule)

	qs.lg.Info(
		"deleted a QoSRule",
		zap.String("QoSRule-name", r.RuleName),
	)
	return &pb.QoSRuleDeleteResponse{}, nil
}

func (qs *qosStore) QoSRuleGet(r *pb.QoSRuleGetRequest) (*pb.QoSRuleGetResponse, error) {
	tx := qs.be.BatchTx()
	tx.Lock()
	var resp pb.QoSRuleGetResponse
	resp.QosRule = getQoSRule(qs.lg, tx, r.RuleName)
	tx.Unlock()

	if resp.QosRule == nil {
		return nil, ErrQoSRuleNotFound
	}
	return &resp, nil
}

func (qs *qosStore) QoSRuleList(r *pb.QoSRuleListRequest) (*pb.QoSRuleListResponse, error) {
	tx := qs.be.BatchTx()
	tx.Lock()
	QoSRules := getAllQoSRules(qs.lg, tx)
	tx.Unlock()
	resp := &pb.QoSRuleListResponse{}
	resp.QosRules = QoSRules
	return resp, nil
}

func getQoSRule(lg *zap.Logger, tx backend.BatchTx, ruleName string) *qospb.QoSRule {
	_, vs := tx.UnsafeRange(qosRuleBucketName, []byte(ruleName), nil, 0)
	if len(vs) == 0 {
		return nil
	}

	QoSRule := &qospb.QoSRule{}
	err := QoSRule.Unmarshal(vs[0])
	if err != nil {
		lg.Panic(
			"failed to unmarshal 'qospb.QoSRule'",
			zap.String("QoSRule-name", ruleName),
			zap.Error(err),
		)
	}
	return QoSRule
}

func getAllQoSRules(lg *zap.Logger, tx backend.BatchTx) []*qospb.QoSRule {
	_, vs := tx.UnsafeRange(qosRuleBucketName, []byte{0}, []byte{0xff}, -1)
	if len(vs) == 0 {
		return nil
	}

	QoSRules := make([]*qospb.QoSRule, len(vs))
	for i := range vs {
		rule := &qospb.QoSRule{}
		err := rule.Unmarshal(vs[i])
		if err != nil {
			lg.Panic("failed to unmarshal 'qospb.QoSRule'", zap.Error(err))
		}
		QoSRules[i] = rule
	}
	return QoSRules
}

func putQoSRule(lg *zap.Logger, tx backend.BatchTx, r *qospb.QoSRule) {
	b, err := r.Marshal()
	if err != nil {
		lg.Panic("failed to unmarshal 'qospb.QoSRule'", zap.Error(err))
	}
	tx.UnsafePut(qosRuleBucketName, []byte(r.RuleName), b)
}

func delQoSRule(tx backend.BatchTx, QoSRulename string) {
	tx.UnsafeDelete(qosRuleBucketName, []byte(QoSRulename))
}

func (qs *qosStore) IsQoSEnabled() bool {
	qs.enabledMu.RLock()
	defer qs.enabledMu.RUnlock()
	return qs.enabled
}

// NewQoSStore creates a new qosStore.
func NewQoSStore(lg *zap.Logger, be backend.Backend, ci cindex.ConsistentIndexer) *qosStore {
	if lg == nil {
		lg = zap.NewNop()
	}

	tx := be.BatchTx()
	tx.Lock()

	tx.UnsafeCreateBucket(qosRuleBucketName)
	tx.UnsafeCreateBucket(qosBucketName)

	enabled := false
	_, vs := tx.UnsafeRange(qosBucketName, enableFlagKey, nil, 0)
	if len(vs) == 1 {
		if bytes.Equal(vs[0], qosEnabled) {
			enabled = true
		}
	}

	qs := &qosStore{
		lg:      lg,
		be:      be,
		ci:      ci,
		enabled: enabled,
	}

	qs.enforcer = NewQoSEnforcer(lg)
	qs.buildQoSRules()

	tx.Unlock()
	be.ForceCommit()

	return qs
}

func (qs *qosStore) buildQoSRules() {
	rules := getAllQoSRules(qs.lg, qs.be.BatchTx())
	for i := 0; i < len(rules); i++ {
		qs.enforcer.SyncRule(rules[i])
	}
}

func (qs *qosStore) GetToken(r *RequestContext) bool {
	if !qs.IsQoSEnabled() {
		return true
	}
	return qs.enforcer.GetToken(r)
}

func (qs *qosStore) PutToken(r *RequestContext) bool {
	if !qs.IsQoSEnabled() {
		return true
	}
	return qs.enforcer.PutToken(r)
}

func (qs *qosStore) saveConsistentIndex(tx backend.BatchTx) {
	if qs.ci != nil {
		qs.ci.UnsafeSave(tx)
	} else {
		qs.lg.Error("failed to save consistentIndex,consistentIndexer is nil")
	}
}
