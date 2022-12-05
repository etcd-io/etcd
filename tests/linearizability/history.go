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

type history struct {
	// id of the next write operation. If needed a new id might be requested from idProvider.
	id         int
	idProvider idProvider

	operations []porcupine.Operation
	failed     []porcupine.Operation
}

func NewHistory(ids idProvider) *history {
	return &history{
		id:         ids.ClientId(),
		idProvider: ids,
		operations: []porcupine.Operation{},
		failed:     []porcupine.Operation{},
	}
}

func (h *history) AppendGet(key string, start, end time.Time, resp *clientv3.GetResponse) {
	var readData string
	if len(resp.Kvs) == 1 {
		readData = string(resp.Kvs[0].Value)
	}
	h.operations = append(h.operations, porcupine.Operation{
		ClientId: h.id,
		Input:    EtcdRequest{Op: Get, Key: key},
		Call:     start.UnixNano(),
		Output:   EtcdResponse{GetData: readData, Revision: resp.Header.Revision},
		Return:   end.UnixNano(),
	})
}

func (h *history) AppendPut(key, value string, start, end time.Time, resp *clientv3.PutResponse, err error) {
	if err != nil {
		h.failed = append(h.failed, porcupine.Operation{
			ClientId: h.id,
			Input:    EtcdRequest{Op: Put, Key: key, PutData: value},
			Call:     start.UnixNano(),
			Output:   EtcdResponse{Err: err},
			Return:   0, // For failed writes we don't know when request has really finished.
		})
		// Operations of single client needs to be sequential.
		// As we don't know return time of failed operations, all new writes need to be done with new client id.
		h.id = h.idProvider.ClientId()
		return
	}
	var revision int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
	}
	h.operations = append(h.operations, porcupine.Operation{
		ClientId: h.id,
		Input:    EtcdRequest{Op: Put, Key: key, PutData: value},
		Call:     start.UnixNano(),
		Output:   EtcdResponse{Err: err, Revision: revision},
		Return:   end.UnixNano(),
	})
}

func (h *history) AppendDelete(key string, start, end time.Time, resp *clientv3.DeleteResponse, err error) {
	if err != nil {
		h.failed = append(h.failed, porcupine.Operation{
			ClientId: h.id,
			Input:    EtcdRequest{Op: Delete, Key: key},
			Call:     start.UnixNano(),
			Output:   EtcdResponse{Err: err},
			Return:   0, // For failed writes we don't know when request has really finished.
		})
		// Operations of single client needs to be sequential.
		// As we don't know return time of failed operations, all new writes need to be done with new client id.
		h.id = h.idProvider.ClientId()
		return
	}
	var revision int64
	var deleted int64
	if resp != nil && resp.Header != nil {
		revision = resp.Header.Revision
		deleted = resp.Deleted
	}
	h.operations = append(h.operations, porcupine.Operation{
		ClientId: h.id,
		Input:    EtcdRequest{Op: Delete, Key: key},
		Call:     start.UnixNano(),
		Output:   EtcdResponse{Revision: revision, Deleted: deleted, Err: err},
		Return:   end.UnixNano(),
	})
}

func (h *history) Operations() []porcupine.Operation {
	operations := make([]porcupine.Operation, 0, len(h.operations)+len(h.failed))
	var maxTime int64
	for _, op := range h.operations {
		operations = append(operations, op)
		if op.Return > maxTime {
			maxTime = op.Return
		}
	}
	for _, op := range h.failed {
		if op.Call > maxTime {
			continue
		}
		op.Return = maxTime + 1
		operations = append(operations, op)
	}
	return operations
}
