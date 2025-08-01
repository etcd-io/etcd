package agent

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"go.etcd.io/etcd/client/pkg/v3/srv"
)

func Endpoints(gcfg GlobalConfig) ([]string, error) {
	if !gcfg.UseClusterEndpoints {
		if len(gcfg.Endpoints) == 0 {
			return nil, errors.New("no endpoints provided")
		}
		return gcfg.Endpoints, nil
	}

	return endpointsFromCluster(gcfg)
}

func endpointsFromCluster(gcfg GlobalConfig) ([]string, error) {
	memberlistResp, err := MemberList(gcfg, nil)
	if err != nil {
		return nil, err
	}

	var eps []string
	for _, m := range memberlistResp.Members {
		eps = append(eps, m.ClientURLs...)
	}

	return eps, nil
}

func endpointsFromCmd(gcfg GlobalConfig) ([]string, error) {
	eps, err := endpointsFromDNSDiscovery(gcfg)
	if err != nil {
		return nil, err
	}

	if len(eps) == 0 {
		eps = gcfg.Endpoints
	}

	if len(eps) == 0 {
		return nil, errors.New("no endpoints provided")
	}

	return eps, nil
}

func endpointsFromDNSDiscovery(gcfg GlobalConfig) ([]string, error) {
	if gcfg.DNSDomain == "" {
		return nil, nil
	}

	srvs, err := srv.GetClient("etcd-client", gcfg.DNSDomain, gcfg.DNSService)
	if err != nil {
		return nil, err
	}

	eps := srvs.Endpoints
	if gcfg.InsecureDiscovery {
		return eps, nil
	}

	// strip insecure connections
	var ret []string
	for _, ep := range eps {
		if strings.HasPrefix(ep, "http://") {
			fmt.Fprintf(os.Stderr, "ignoring discovered insecure endpoint %q\n", ep)
			continue
		}
		ret = append(ret, ep)
	}
	return ret, nil
}
