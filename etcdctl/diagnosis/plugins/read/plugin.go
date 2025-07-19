package read

import (
	"log"
	"time"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"

	"go.etcd.io/etcd/etcdctl/v3/diagnosis/agent"
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

func NewPlugin(gcfg agent.GlobalConfig, linearizable bool) intf.Plugin {
	return &readChecker{
		Checker: common.Checker{
			GlobalConfig: gcfg,
			Name:         generateName(linearizable),
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
	var (
		eps []string
		err error
	)

	defer func() {
		if err != nil {
			result = &intf.FailedResult{
				Name:   ck.Name(),
				Reason: err.Error(),
			}
		}
	}()

	if eps, err = agent.Endpoints(ck.GlobalConfig); err != nil {
		log.Printf("Failed to get endpoint: %v\n", err)
		return
	}
	log.Printf("Endpoints: %v\n", eps)

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
			var err error
			if ck.linearizable {
				_, err = agent.Read(ck.GlobalConfig, []string{ep}, "health")
			} else {
				_, err = agent.Read(ck.GlobalConfig, []string{ep}, "health", clientv3.WithSerializable())
			}
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
