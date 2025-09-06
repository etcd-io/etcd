package read

import (
	"context"
	"log"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"

	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/common"
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
			if err != nil && err != rpctypes.ErrPermissionDenied {
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
	return
}

func initCheckResult(name string, epCount int) checkResult {
	return checkResult{
		Name:          name,
		Summary:       "",
		ReadResponses: make([]readResponse, epCount),
	}
}
