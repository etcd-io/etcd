// Copyright 2022 The etcd Authors
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

package linearizability

import (
	"time"

	"github.com/anishathalye/porcupine"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type appendableHistory struct {
	// id of the next write operation. If needed a new id might be requested from idProvider.
	id         int
	idProvider idProvider

	history
}

func newAppendableHistory(ids idProvider) *appendableHistory {
	return &appendableHistory{
		id:         ids.ClientId(),
		idProvider: ids,
		history: history{
			successful: []porcupine.Operation{},
			failed:     []porcupine.Operation{},
		},
	}
}

func (h *appendableHistory) AppendGet(key string, start, end time.Time, resp *clientv3.GetResponse) {
	var readData string
	if len(resp.Kvs) == 1 {
		readData = string(resp.Kvs[0].Value)
	}
	h.successful = append(h.successful, porcupine.Operation{
		ClientId: h.id,
		Input:    EtcdRequest{Op: Get, Key: key},
		Call:     start.UnixNano(),
		Output:   EtcdResponse{GetData: readData, Revision: resp.Header.Revision},
		Return:   end.UnixNano(),
	})
}

func (h *appendableHistory) AppendPut(key, value string, start, end time.Time, resp *clientv3.PutResponse, err error) {
	request := EtcdRequest{Op: Put, Key: key, PutData: value}
	if err != nil {
		h.appendFailed(request, start, err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.successful = append(h.successful, porcupine.Operation{
		ClientId: h.id,
		Input:    EtcdRequest{Op: Put, Key: key, PutData: value},
		Call:     start.UnixNano(),
		Output:   EtcdResponse{Err: err, Revision: revision},
		Return:   end.UnixNano(),
	})
}

func (h *appendableHistory) AppendDelete(key string, start, end time.Time, resp *clientv3.DeleteResponse, err error) {
	request := EtcdRequest{Op: Delete, Key: key}
	if err != nil {
		h.appendFailed(request, start, err)
		return
	}
	var revision int64
	var deleted int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
		deleted = resp.Deleted
	}
	h.successful = append(h.successful, porcupine.Operation{
		ClientId: h.id,
		Input:    request,
		Call:     start.UnixNano(),
		Output:   EtcdResponse{Revision: revision, Deleted: deleted, Err: err},
		Return:   end.UnixNano(),
	})
}

func (h *appendableHistory) AppendTxn(key, expectValue, newValue string, start, end time.Time, resp *clientv3.TxnResponse, err error) {
	request := EtcdRequest{Op: Txn, Key: key, TxnExpectData: expectValue, TxnNewData: newValue}
	if err != nil {
		h.appendFailed(request, start, err)
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.successful = append(h.successful, porcupine.Operation{
		ClientId: h.id,
		Input:    request,
		Call:     start.UnixNano(),
		Output:   EtcdResponse{Err: err, Revision: revision, TxnSucceeded: resp.Succeeded},
		Return:   end.UnixNano(),
	})
}

func (h *appendableHistory) appendFailed(request EtcdRequest, start time.Time, err error) {
	h.failed = append(h.failed, porcupine.Operation{
		ClientId: h.id,
		Input:    request,
		Call:     start.UnixNano(),
		Output:   EtcdResponse{Err: err},
		Return:   0, // For failed writes we don't know when request has really finished.
	})
	// Operations of single client needs to be sequential.
	// As we don't know return time of failed operations, all new writes need to be done with new client id.
	h.id = h.idProvider.ClientId()
}

type history struct {
	successful []porcupine.Operation
	// failed requests are kept separate as we don't know return time of failed operations.
	// Based on https://github.com/anishathalye/porcupine/issues/10
	failed []porcupine.Operation
}

func (h history) Merge(h2 history) history {
	result := history{
		successful: make([]porcupine.Operation, 0, len(h.successful)+len(h2.successful)),
		failed:     make([]porcupine.Operation, 0, len(h.failed)+len(h2.failed)),
	}
	result.successful = append(result.successful, h.successful...)
	result.successful = append(result.successful, h2.successful...)
	result.failed = append(result.failed, h.failed...)
	result.failed = append(result.failed, h2.failed...)
	return result
}

func (h history) Operations() []porcupine.Operation {
	operations := make([]porcupine.Operation, 0, len(h.successful)+len(h.failed))
	var maxTime int64
	for _, op := range h.successful {
		operations = append(operations, op)
		if op.Return > maxTime {
			maxTime = op.Return
		}
	}
	// Failed requests don't have a known return time.
	// We simulate Infinity by using return time of latest successfully request.
	for _, op := range h.failed {
		if op.Call > maxTime {
			continue
		}
		op.Return = maxTime + 1
		operations = append(operations, op)
	}
	return operations
}
