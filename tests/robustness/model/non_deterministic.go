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

package model

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/anishathalye/porcupine"
)

// NonDeterministicModel extends DeterministicModel to allow for clients with imperfect knowledge of the request's destiny.
// An unknown/error response doesn't inform whether the request was persisted or not, so the model
// considers both cases. This is represented as multiple, equally possible deterministic states.
// Failed requests fork the possible states, while successful requests merge and filter them.
var NonDeterministicModel = func(keys []string) porcupine.Model {
	return porcupine.Model{
		Init: func() any {
			return nonDeterministicState{freshEtcdState(keys)}
		},
		StepContext: func(ctx context.Context, st any, in any, out any) (bool, any) {
			return st.(nonDeterministicState).applyContext(ctx, in.(EtcdRequest), out.(MaybeEtcdResponse))
		},
		Equal: func(st1, st2 any) bool {
			return st1.(nonDeterministicState).Equal(st2.(nonDeterministicState))
		},
		DescribeOperation: func(in, out any) string {
			return fmt.Sprintf("%s -> %s", describeEtcdRequest(in.(EtcdRequest)), describeEtcdResponse(in.(EtcdRequest), out.(MaybeEtcdResponse)))
		},
		DescribeOperationMetadata: func(info any) string {
			if info == nil {
				return ""
			}
			return DescribeOperationMetadata(info.(MaybeEtcdResponse))
		},
		DescribeState: func(st any) string {
			etcdStates := st.(nonDeterministicState)
			desc := make([]string, 0, len(etcdStates))

			slices.SortFunc(etcdStates, compareStates)

			for i, s := range etcdStates {
				// Describe just 3 first states before truncating
				if i >= 3 {
					desc = append(desc, "...truncated...")
					break
				}
				desc = append(desc, describeEtcdState(s))
			}

			return strings.Join(desc, "\n")
		},
	}
}

type nonDeterministicState []EtcdState

func (states nonDeterministicState) Equal(other nonDeterministicState) bool {
	if len(states) != len(other) {
		return false
	}
	slices.SortFunc(states, compareStates)
	slices.SortFunc(other, compareStates)

	for i := range states {
		if !states[i].Equal(other[i]) {
			return false
		}
	}
	return true
}

func compareStates(first, second EtcdState) int {
	if c := cmp.Compare(first.Revision, second.Revision); c != 0 {
		return c
	}
	if c := cmp.Compare(first.CompactRevision, second.CompactRevision); c != 0 {
		return c
	}
	if c := cmp.Compare(len(first.KeyValues), len(second.KeyValues)); c != 0 {
		return c
	}
	for i := range first.KeyValues {
		if (first.KeyValues[i] == nil) != (second.KeyValues[i] == nil) {
			if first.KeyValues[i] == nil {
				return -1
			}
			return 1
		}
		if first.KeyValues[i] == nil {
			continue
		}
		if c := cmp.Compare(first.KeyValues[i].ModRevision, second.KeyValues[i].ModRevision); c != 0 {
			return c
		}
		if c := cmp.Compare(first.KeyValues[i].Version, second.KeyValues[i].Version); c != 0 {
			return c
		}
		if c := cmp.Compare(first.KeyValues[i].Value.Value, second.KeyValues[i].Value.Value); c != 0 {
			return c
		}
		if c := cmp.Compare(first.KeyValues[i].Value.Hash, second.KeyValues[i].Value.Hash); c != 0 {
			return c
		}
	}
	if c := cmp.Compare(len(first.KeyLeases), len(second.KeyLeases)); c != 0 {
		return c
	}
	for i := range first.KeyLeases {
		if (first.KeyLeases[i] == nil) != (second.KeyLeases[i] == nil) {
			if first.KeyLeases[i] == nil {
				return -1
			}
			return 1
		}
		if first.KeyLeases[i] == nil {
			continue
		}
		if c := cmp.Compare(*first.KeyLeases[i], *second.KeyLeases[i]); c != 0 {
			return c
		}
	}
	if c := cmp.Compare(len(first.Leases), len(second.Leases)); c != 0 {
		return c
	}
	for i := range first.Leases {
		if c := cmp.Compare(first.Leases[i], second.Leases[i]); c != 0 {
			return c
		}
	}
	return 0
}

func (states nonDeterministicState) applyContext(ctx context.Context, request EtcdRequest, response MaybeEtcdResponse) (bool, nonDeterministicState) {
	if ctx.Err() != nil {
		return false, nil
	}
	var newStates nonDeterministicState
	switch {
	case response.Error != "":
		newStates = states.applyFailedRequestContext(ctx, request)
	case response.Persisted && response.PersistedRevision == 0:
		newStates = states.applyPersistedRequestContext(ctx, request)
	case response.Persisted && response.PersistedRevision != 0:
		newStates = states.applyPersistedRequestWithRevisionContext(ctx, request, response.PersistedRevision)
	default:
		newStates = states.applyRequestWithResponseContext(ctx, request, response.EtcdResponse)
	}
	if ctx.Err() != nil {
		return false, nil
	}
	return len(newStates) > 0, newStates
}

// applyFailedRequestContext returns both the original states and states with applied request. It considers both cases, request was persisted and request was lost.
func (states nonDeterministicState) applyFailedRequestContext(ctx context.Context, request EtcdRequest) nonDeterministicState {
	if ctx.Err() != nil {
		return nil
	}
	newStates := make(nonDeterministicState, 0, len(states)*2)
	for _, s := range states {
		if ctx.Err() != nil {
			return nil
		}
		newStates = append(newStates, s)
		newState, _ := s.Step(request)
		if ctx.Err() != nil {
			return nil
		}
		if !newState.Equal(s) {
			if ctx.Err() != nil {
				return nil
			}
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applyPersistedRequestContext applies request to all possible states.
func (states nonDeterministicState) applyPersistedRequestContext(ctx context.Context, request EtcdRequest) nonDeterministicState {
	if ctx.Err() != nil {
		return nil
	}
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		if ctx.Err() != nil {
			return nil
		}
		newState, _ := s.Step(request)
		if ctx.Err() != nil {
			return nil
		}
		newStates = append(newStates, newState)
	}
	return newStates
}

// applyPersistedRequestWithRevisionContext applies request to all possible states, but leaves only states that would return proper revision.
func (states nonDeterministicState) applyPersistedRequestWithRevisionContext(ctx context.Context, request EtcdRequest, responseRevision int64) nonDeterministicState {
	if ctx.Err() != nil {
		return nil
	}
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		if ctx.Err() != nil {
			return nil
		}
		newState, modelResponse := s.Step(request)
		if ctx.Err() != nil {
			return nil
		}
		if modelResponse.Revision == responseRevision {
			newStates = append(newStates, newState)
		}
	}
	return newStates
}

// applyRequestWithResponseContext applies request to all possible states, but leaves only state that would return proper response.
func (states nonDeterministicState) applyRequestWithResponseContext(ctx context.Context, request EtcdRequest, response EtcdResponse) nonDeterministicState {
	if ctx.Err() != nil {
		return nil
	}
	newStates := make(nonDeterministicState, 0, len(states))
	for _, s := range states {
		if ctx.Err() != nil {
			return nil
		}
		newState, modelResponse := s.Step(request)
		if ctx.Err() != nil {
			return nil
		}
		if Match(modelResponse, MaybeEtcdResponse{EtcdResponse: response}) {
			if ctx.Err() != nil {
				return nil
			}
			newStates = append(newStates, newState)
		}
	}
	return newStates
}
