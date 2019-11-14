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

package client

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/golang/protobuf/proto"
	structpb "github.com/golang/protobuf/ptypes/struct"
	basepb "google.golang.org/grpc/xds/internal/proto/envoy/api/v2/core/base"
)

// TestNewConfig exercises the functionality in NewConfig with different
// bootstrap file contents. It overrides the fileReadFunc by returning
// bootstrap file contents defined in this test, instead of reading from a
// file. It also overrides onceDoerFunc to disable reading the bootstrap file
// only once.
func TestNewConfig(t *testing.T) {
	bootstrapFileMap := map[string]string{
		"empty":   "",
		"badJSON": `["test": 123]`,
		"badNodeProto": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"badField": "foobar",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			}
		}`,
		"badApiConfigSourceProto": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			},
			"xds_server" : {
			    "api_type": "GRPC",
				"badField": "foobar",
			    "grpc_services": [
					{
						"google_grpc": {
							"target_uri": "trafficdirector.googleapis.com:443"
						}
					}
				]
			}
		}`,
		"badTopLevelFieldInFile": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			},
			"xds_server" : {
			    "api_type": "GRPC",
			    "grpc_services": [
					{
						"google_grpc": {
							"target_uri": "trafficdirector.googleapis.com:443"
						}
					}
				]
			},
			"badField": "foobar"
		}`,
		"emptyNodeProto": `
		{
			"xds_server" : {
			    "api_type": "GRPC",
			    "grpc_services": [
					{
						"google_grpc": {
							"target_uri": "trafficdirector.googleapis.com:443"
						}
					}
				]
			}
		}`,
		"emptyApiConfigSourceProto": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			}
		}`,
		"badApiTypeInFile": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			},
			"xds_server" : {
			    "api_type": "REST",
			    "grpc_services": [
					{
						"google_grpc": {
							"target_uri": "trafficdirector.googleapis.com:443"
						}
					}
				]
			}
		}`,
		"noGrpcServices": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			},
			"xds_server" : {
			    "api_type": "GRPC",
			}
		}`,
		"tooManyGrpcServices": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			},
			"xds_server" : {
			    "api_type": "GRPC",
			    "grpc_services": [
					{
						"google_grpc": {
							"target_uri": "trafficdirector.googleapis.com:443"
						}
					},
					{
						"google_grpc": {
							"target_uri": "foobar.googleapis.com:443"
						}
					}
				]
			}
		}`,
		"goodBootstrap": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			},
			"xds_server" : {
			    "api_type": "GRPC",
			    "grpc_services": [
					{
						"google_grpc": {
							"target_uri": "trafficdirector.googleapis.com:443"
						}
					}
				]
			}
		}`,
	}

	oldFileReadFunc := fileReadFunc
	fileReadFunc = func(name string) ([]byte, error) {
		if b, ok := bootstrapFileMap[name]; ok {
			return []byte(b), nil
		}
		return nil, os.ErrNotExist
	}
	oldOnceDoerFunc := onceDoerFunc
	onceDoerFunc = func(f func()) {
		// Disable the synce.Once functionality to read the bootstrap file.
		// Instead, read it everytime NewConfig() is called so that we can test
		// with different file contents.
		f()
	}
	defer func() {
		fileReadFunc = oldFileReadFunc
		onceDoerFunc = oldOnceDoerFunc
		os.Unsetenv(bootstrapFileEnv)
	}()

	tests := []struct {
		name             string
		fName            string
		wantErr          bool
		wantBalancerName string
		wantNodeProto    *basepb.Node
		// TODO: It doesn't look like there is an easy way to compare the value
		// stored in Creds with an expected value. Figure out a way to make it
		// testable.
	}{
		{
			name:             "non-existent-bootstrap-file",
			fName:            "dummy",
			wantErr:          true,
			wantBalancerName: "",
			wantNodeProto:    nil,
		},
		{
			name:             "bad-json-in-file",
			fName:            "badJSON",
			wantErr:          true,
			wantBalancerName: "",
			wantNodeProto:    nil,
		},
		{
			name:             "bad-nodeProto-in-file",
			fName:            "badNodeProto",
			wantErr:          true,
			wantBalancerName: "",
			wantNodeProto:    nil,
		},
		{
			name:             "bad-ApiConfigSourceProto-in-file",
			fName:            "badApiConfigSourceProto",
			wantErr:          true,
			wantBalancerName: "",
			wantNodeProto:    nil,
		},
		{
			name:             "bad-top-level-field-in-file",
			fName:            "badTopLevelFieldInFile",
			wantErr:          true,
			wantBalancerName: "",
			wantNodeProto:    nil,
		},
		{
			name:             "empty-nodeProto-in-file",
			fName:            "emptyNodeProto",
			wantErr:          true,
			wantBalancerName: "",
			wantNodeProto:    nil,
		},
		{
			name:             "empty-apiConfigSourceProto-in-file",
			fName:            "emptyApiConfigSourceProto",
			wantErr:          true,
			wantBalancerName: "",
			wantNodeProto:    nil,
		},
		{
			name:             "bad-api-type-in-file",
			fName:            "badApiTypeInFile",
			wantErr:          true,
			wantBalancerName: "",
			wantNodeProto:    nil,
		},
		{
			name:             "good-bootstrap",
			fName:            "goodBootstrap",
			wantErr:          false,
			wantBalancerName: "trafficdirector.googleapis.com:443",
			wantNodeProto: &basepb.Node{
				Id: "ENVOY_NODE_ID",
				Metadata: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"TRAFFICDIRECTOR_GRPC_HOSTNAME": {
							Kind: &structpb.Value_StringValue{StringValue: "trafficdirector"},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		if err := os.Setenv(bootstrapFileEnv, test.fName); err != nil {
			t.Fatalf("%s: os.Setenv(%s, %s) failed with error: %v", test.name, bootstrapFileEnv, test.fName, err)
		}
		config, err := NewConfig()
		if (err != nil) != test.wantErr {
			t.Fatalf("%s: NewConfig() returned error: %v, wantErr: %v", test.name, err, test.wantErr)
		}
		if !test.wantErr {
			if got := config.BalancerName; got != test.wantBalancerName {
				t.Errorf("%s: config.BalancerName is %s, want %s", test.name, got, test.wantBalancerName)
			}
			if got := config.NodeProto; !proto.Equal(got, test.wantNodeProto) {
				t.Errorf("%s: config.NodeProto is %#v, want %#v", test.name, got, test.wantNodeProto)
			}
		}
	}
}

// TestNewConfigOnce does not override onceDoerFunc, which means that the
// bootstrap file will be read only once. This test first supplies a bad
// bootstrap file and makes sure that the error from reading the bootstrap file
// is stored in package level vars and returned on subsequent calls to
// NewConfig.
func TestNewConfigOnce(t *testing.T) {
	// This test could be executed multiple times as part of the same test
	// binary (especially in cases where we pass in different values for the
	// -cpu flag). We want each run to start off with a fresh state.
	bsOnce = sync.Once{}

	bootstrapFileMap := map[string]string{
		"badJSON": `["test": 123]`,
		"goodBootstrap": `
		{
			"node": {
				"id": "ENVOY_NODE_ID",
				"metadata": {
				    "TRAFFICDIRECTOR_GRPC_HOSTNAME": "trafficdirector"
			    }
			},
			"xds_server" : {
			    "api_type": "GRPC",
			    "grpc_services": [
					{
						"google_grpc": {
							"target_uri": "trafficdirector.googleapis.com:443"
						}
					}
				]
			}
		}`,
	}

	oldFileReadFunc := fileReadFunc
	fileReadFunc = func(name string) ([]byte, error) {
		if b, ok := bootstrapFileMap[name]; ok {
			return []byte(b), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() {
		fileReadFunc = oldFileReadFunc
		os.Unsetenv(bootstrapFileEnv)
	}()

	// Pass bad JSON in bootstrap file. This should end up being stored in
	// package level vars and returned in further calls to NewConfig.
	if err := os.Setenv(bootstrapFileEnv, "badJSON"); err != nil {
		t.Fatalf("os.Setenv(%s, badJSON) failed with error: %v", bootstrapFileEnv, err)
	}
	if _, err := NewConfig(); err == nil || !strings.Contains(err.Error(), "json.Unmarshal") {
		t.Fatalf("NewConfig() returned error: %v, want json.Unmarshal error", err)
	}

	// Setting the bootstrap file to valid contents should not succeed, as the
	// file is read only on the first call to NewConfig.
	if err := os.Setenv(bootstrapFileEnv, "goodBootstrap"); err != nil {
		t.Fatalf("os.Setenv(%s, badJSON) failed with error: %v", bootstrapFileEnv, err)
	}
	if _, err := NewConfig(); err == nil || !strings.Contains(err.Error(), "json.Unmarshal") {
		t.Fatalf("NewConfig() returned error: %v, want json.Unmarshal error", err)
	}
}
