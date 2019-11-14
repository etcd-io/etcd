/*
 *
 * Copyright 2019 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package client contains the implementation of the xds client used by
// grpc-lb-v2.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/google"
	"google.golang.org/grpc/grpclog"
	basepb "google.golang.org/grpc/xds/internal/proto/envoy/api/v2/core/base"
	cspb "google.golang.org/grpc/xds/internal/proto/envoy/api/v2/core/config_source"
)

// Environment variable which holds the name of the xDS bootstrap file.
const bootstrapFileEnv = "GRPC_XDS_BOOTSTRAP"

var (
	// Bootstrap file is read once (on the first invocation of NewConfig), and
	// the results are stored in these package level vars.
	bsOnce sync.Once
	bsData *bootstrapData
	bsErr  error
)

// For overriding from unit tests.
var (
	fileReadFunc = ioutil.ReadFile
	onceDoerFunc = bsOnce.Do
)

// Config provides the xDS client with several key bits of information that it
// requires in its interaction with an xDS server. The Config is initialized
// from the bootstrap file. If that process fails for any reason, it uses the
// defaults passed in.
type Config struct {
	// BalancerName is the name of the xDS server to connect to.
	BalancerName string
	// Creds contains the credentials to be used while talking to the xDS
	// server, as a grpc.DialOption.
	Creds grpc.DialOption
	// NodeProto contains the basepb.Node proto to be used in xDS calls made to the
	// server.
	NodeProto *basepb.Node
}

// NewConfig returns a new instance of Config initialized by reading the
// bootstrap file found at ${GRPC_XDS_BOOTSTRAP}. Bootstrap file is read only
// on the first invocation of this function, and further invocations end up
// using the results from the former.
//
// As of today, the bootstrap file only provides the balancer name and the node
// proto to be used in calls to the balancer. For transport credentials, the
// default TLS config with system certs is used. For call credentials, default
// compute engine credentials are used.
func NewConfig() (*Config, error) {
	onceDoerFunc(func() {
		fName, ok := os.LookupEnv(bootstrapFileEnv)
		if !ok {
			bsData, bsErr = nil, fmt.Errorf("xds: %s environment variable not set", bootstrapFileEnv)
			return
		}
		bsData, bsErr = readBootstrapFile(fName)
	})
	if bsErr != nil {
		return nil, bsErr
	}
	return &Config{
		BalancerName: bsData.balancerName(),
		Creds:        grpc.WithCredentialsBundle(google.NewComputeEngineCredentials()),
		NodeProto:    bsData.node,
	}, nil
}

// bootstrapData wraps the contents of the bootstrap file.
// Today the bootstrap file contains a Node proto and an ApiConfigSource proto
// in JSON format.
type bootstrapData struct {
	node      *basepb.Node
	xdsServer *cspb.ApiConfigSource
}

func (bsd *bootstrapData) balancerName() string {
	// If the bootstrap file was read and parsed successfully, this should be
	// setup properly. So, we skip checking for the presence of these fields
	// before accessing and returning it.
	return bsd.xdsServer.GetGrpcServices()[0].GetGoogleGrpc().GetTargetUri()
}

func readBootstrapFile(name string) (*bootstrapData, error) {
	grpclog.Infof("xds: Reading bootstrap file from %s", name)
	data, err := fileReadFunc(name)
	if err != nil {
		return nil, fmt.Errorf("xds: bootstrap file {%v} read failed: %v", name, err)
	}

	var jsonData map[string]json.RawMessage
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("xds: json.Unmarshal(%v) failed during bootstrap: %v", string(data), err)
	}

	bsd := &bootstrapData{}
	m := jsonpb.Unmarshaler{}
	for k, v := range jsonData {
		switch k {
		case "node":
			n := &basepb.Node{}
			if err := m.Unmarshal(bytes.NewReader(v), n); err != nil {
				return nil, fmt.Errorf("xds: jsonpb.Unmarshal(%v) failed during bootstrap: %v", string(v), err)
			}
			bsd.node = n
		case "xds_server":
			s := &cspb.ApiConfigSource{}
			if err := m.Unmarshal(bytes.NewReader(v), s); err != nil {
				return nil, fmt.Errorf("xds: jsonpb.Unmarshal(%v) failed during bootstrap: %v", string(v), err)
			}
			bsd.xdsServer = s
		default:
			return nil, fmt.Errorf("xds: unexpected data in bootstrap file: {%v, %v}", k, string(v))
		}
	}

	if bsd.node == nil || bsd.xdsServer == nil {
		return nil, fmt.Errorf("xds: incomplete data in bootstrap file: %v", string(data))
	}

	if api := bsd.xdsServer.GetApiType(); api != cspb.ApiConfigSource_GRPC {
		return nil, fmt.Errorf("xds: apiType in bootstrap file is %v, want GRPC", api)
	}
	if n := len(bsd.xdsServer.GetGrpcServices()); n != 1 {
		return nil, fmt.Errorf("xds: %v grpc services listed in bootstrap file, want 1", n)
	}
	if bsd.xdsServer.GetGrpcServices()[0].GetGoogleGrpc().GetTargetUri() == "" {
		return nil, fmt.Errorf("xds: trafficdirector name missing in bootstrap file")
	}

	return bsd, nil
}
