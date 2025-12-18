// Copyright 2025 The etcd Authors
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

package read

import (
	"context"
	"errors"
	"log"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/plugins/common"
)

type readChecker struct {
	common.Checker
	linearizable bool
}

type readResponse struct {
	Endpoint string `json:"endpoint,omitempty"`
	Took     string `json:"took,omitempty"`
	Error    string `json:"error,omitempty"`
}
type checkResult struct {
	Name          string         `json:"name,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	ReadResponses []readResponse `json:"readResponses,omitempty"`
}

func NewPlugin(cfg *clientv3.ConfigSpec, eps []string, timeout time.Duration, linearizable bool) intf.Plugin {
	return &readChecker{
		Checker: common.Checker{
			Cfg:            cfg,
			Endpoints:      eps,
			CommandTimeout: timeout,
			Name:           generateName(linearizable),
		},
		linearizable: linearizable,
	}
}

func (ck *readChecker) Name() string {
	return ck.Checker.Name
}

func generateName(linearizable bool) string {
	if linearizable {
		return "linearizableReadChecker"
	}
	return "serializableReadChecker"
}

func (ck *readChecker) Diagnose() (result any) {
	var err error
	eps := ck.Endpoints

	defer func() {
		if err != nil {
			result = &intf.FailedResult{
				Name:   ck.Name(),
				Reason: err.Error(),
			}
		}
	}()

	var (
		maxRetries = 3
		retries    = 0

		chkResult = initCheckResult(ck.Name(), len(eps))
	)

	for {
		shouldRetry := false
		for i, ep := range eps {
			chkResult.ReadResponses[i].Endpoint = ep

			startTs := time.Now()
			cfg := common.ConfigWithEndpoint(ck.Cfg, ep)
			c, err := common.NewClient(cfg)
			if err != nil {
				chkResult.ReadResponses[i].Error = err.Error()
				shouldRetry = true
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), ck.CommandTimeout)
			if ck.linearizable {
				_, err = c.Get(ctx, "health")
			} else {
				_, err = c.Get(ctx, "health", clientv3.WithSerializable())
			}
			cancel()
			c.Close()
			if err != nil && !errors.Is(err, rpctypes.ErrPermissionDenied) {
				chkResult.ReadResponses[i].Error = err.Error()
				shouldRetry = true
			}
			chkResult.ReadResponses[i].Took = time.Since(startTs).String()
		}

		retries++

		if !shouldRetry || retries >= maxRetries {
			break
		}

		chkResult = initCheckResult(ck.Name(), len(eps))
		log.Printf("Retrying checking read: %d/%d\n", retries, maxRetries)
		time.Sleep(time.Second)
	}

	chkResult.Summary = "Successful"
	for _, resp := range chkResult.ReadResponses {
		if len(resp.Error) > 0 {
			chkResult.Summary = "Unsuccessful"
			break
		}
	}

	result = chkResult
	return result
}

func initCheckResult(name string, epCount int) checkResult {
	return checkResult{
		Name:          name,
		Summary:       "",
		ReadResponses: make([]readResponse, epCount),
	}
}
