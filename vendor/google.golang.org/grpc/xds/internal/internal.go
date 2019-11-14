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
 */

package internal

import (
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/serviceconfig"
	basepb "google.golang.org/grpc/xds/internal/proto/envoy/api/v2/core/base"
)

// Locality is xds.Locality without XXX fields, so it can be used as map
// keys.
//
// xds.Locality cannot be map keys because one of the XXX fields is a slice.
//
// This struct should only be used as map keys. Use the proto message directly
// in all other places.
type Locality struct {
	Region  string
	Zone    string
	SubZone string
}

func (lamk Locality) String() string {
	return fmt.Sprintf("%s-%s-%s", lamk.Region, lamk.Zone, lamk.SubZone)
}

// ToProto convert Locality to the proto representation.
func (lamk Locality) ToProto() *basepb.Locality {
	return &basepb.Locality{
		Region:  lamk.Region,
		Zone:    lamk.Zone,
		SubZone: lamk.SubZone,
	}
}

// LBConfig represents the loadBalancingConfig section of the service config
// for xDS balancers.
type LBConfig struct {
	serviceconfig.LoadBalancingConfig
	// BalancerName represents the load balancer to use.
	BalancerName string
	// ChildPolicy represents the load balancing config for the child policy.
	ChildPolicy *LoadBalancingConfig
	// FallBackPolicy represents the load balancing config for the fallback.
	FallBackPolicy *LoadBalancingConfig
}

// UnmarshalJSON parses the JSON-encoded byte slice in data and stores it in l.
// When unmarshalling, we iterate through the childPolicy/fallbackPolicy lists
// and select the first LB policy which has been registered.
func (l *LBConfig) UnmarshalJSON(data []byte) error {
	var val map[string]json.RawMessage
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	for k, v := range val {
		switch k {
		case "balancerName":
			if err := json.Unmarshal(v, &l.BalancerName); err != nil {
				return err
			}
		case "childPolicy":
			var lbcfgs []*LoadBalancingConfig
			if err := json.Unmarshal(v, &lbcfgs); err != nil {
				return err
			}
			for _, lbcfg := range lbcfgs {
				if balancer.Get(lbcfg.Name) != nil {
					l.ChildPolicy = lbcfg
					break
				}
			}
		case "fallbackPolicy":
			var lbcfgs []*LoadBalancingConfig
			if err := json.Unmarshal(v, &lbcfgs); err != nil {
				return err
			}
			for _, lbcfg := range lbcfgs {
				if balancer.Get(lbcfg.Name) != nil {
					l.FallBackPolicy = lbcfg
					break
				}
			}
		}
	}
	return nil
}

// MarshalJSON returns a JSON enconding of l.
func (l *LBConfig) MarshalJSON() ([]byte, error) {
	return nil, nil
}

// LoadBalancingConfig represents a single load balancing config,
// stored in JSON format.
type LoadBalancingConfig struct {
	Name   string
	Config json.RawMessage
}

// MarshalJSON returns a JSON enconding of l.
func (l *LoadBalancingConfig) MarshalJSON() ([]byte, error) {
	m := make(map[string]json.RawMessage)
	m[l.Name] = l.Config
	return json.Marshal(m)
}

// UnmarshalJSON parses the JSON-encoded byte slice in data and stores it in l.
func (l *LoadBalancingConfig) UnmarshalJSON(data []byte) error {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	for name, config := range cfg {
		l.Name = name
		l.Config = config
	}
	return nil
}
