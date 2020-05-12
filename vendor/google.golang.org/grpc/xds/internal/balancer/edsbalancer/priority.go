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

package edsbalancer

import (
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/grpclog"
)

// handlePriorityChange handles priority after EDS adds/removes a
// priority.
//
// - If all priorities were deleted, unset priorityInUse, and set parent
// ClientConn to TransientFailure
// - If priorityInUse wasn't set, this is either the first EDS resp, or the
// previous EDS resp deleted everything. Set priorityInUse to 0, and start 0.
// - If priorityInUse was deleted, send the picker from the new lowest priority
// to parent ClientConn, and set priorityInUse to the new lowest.
// - If priorityInUse has a non-Ready state, and also there's a priority lower
// than priorityInUse (which means a lower priority was added), set the next
// priority as new priorityInUse, and start the bg.
func (xdsB *EDSBalancer) handlePriorityChange() {
	xdsB.priorityMu.Lock()
	defer xdsB.priorityMu.Unlock()

	// Everything was removed by EDS.
	if !xdsB.priorityLowest.isSet() {
		xdsB.priorityInUse = newPriorityTypeUnset()
		xdsB.cc.UpdateState(balancer.State{ConnectivityState: connectivity.TransientFailure, Picker: base.NewErrPickerV2(balancer.ErrTransientFailure)})
		return
	}

	// priorityInUse wasn't set, use 0.
	if !xdsB.priorityInUse.isSet() {
		xdsB.startPriority(newPriorityType(0))
		return
	}

	// priorityInUse was deleted, use the new lowest.
	if _, ok := xdsB.priorityToLocalities[xdsB.priorityInUse]; !ok {
		xdsB.priorityInUse = xdsB.priorityLowest
		if s, ok := xdsB.priorityToState[xdsB.priorityLowest]; ok {
			xdsB.cc.UpdateState(*s)
		} else {
			// If state for priorityLowest is not found, this means priorityLowest was
			// started, but never sent any update. The init timer fired and
			// triggered the next priority. The old_priorityInUse (that was just
			// deleted EDS) was picked later.
			//
			// We don't have an old state to send to parent, but we also don't
			// want parent to keep using picker from old_priorityInUse. Send an
			// update to trigger block picks until a new picker is ready.
			xdsB.cc.UpdateState(balancer.State{ConnectivityState: connectivity.Connecting, Picker: base.NewErrPickerV2(balancer.ErrNoSubConnAvailable)})
		}
		return
	}

	// priorityInUse is not ready, look for next priority, and use if found.
	if s, ok := xdsB.priorityToState[xdsB.priorityInUse]; ok && s.ConnectivityState != connectivity.Ready {
		pNext := xdsB.priorityInUse.nextLower()
		if _, ok := xdsB.priorityToLocalities[pNext]; ok {
			xdsB.startPriority(pNext)
		}
	}
}

// startPriority sets priorityInUse to p, and starts the balancer group for p.
// It also starts a timer to fall to next priority after timeout.
//
// Caller must hold priorityMu, priority must exist, and xdsB.priorityInUse must
// be non-nil.
func (xdsB *EDSBalancer) startPriority(priority priorityType) {
	xdsB.priorityInUse = priority
	p := xdsB.priorityToLocalities[priority]
	// NOTE: this will eventually send addresses to sub-balancers. If the
	// sub-balancer tries to update picker, it will result in a deadlock on
	// priorityMu. But it's not an expected behavior for the balancer to
	// update picker when handling addresses.
	p.bg.start()
	// startPriority can be called when
	// 1. first EDS resp, start p0
	// 2. a high priority goes Failure, start next
	// 3. a high priority init timeout, start next
	//
	// In all the cases, the existing init timer is either closed, also already
	// expired. There's no need to close the old timer.
	xdsB.priorityInitTimer = time.AfterFunc(defaultPriorityInitTimeout, func() {
		xdsB.priorityMu.Lock()
		defer xdsB.priorityMu.Unlock()
		if !xdsB.priorityInUse.equal(priority) {
			return
		}
		xdsB.priorityInitTimer = nil
		pNext := priority.nextLower()
		if _, ok := xdsB.priorityToLocalities[pNext]; ok {
			xdsB.startPriority(pNext)
		}
	})
}

// handlePriorityWithNewState start/close priorities based on the connectivity
// state. It returns whether the state should be forwarded to parent ClientConn.
func (xdsB *EDSBalancer) handlePriorityWithNewState(priority priorityType, s balancer.State) bool {
	xdsB.priorityMu.Lock()
	defer xdsB.priorityMu.Unlock()

	if !xdsB.priorityInUse.isSet() {
		grpclog.Infof("eds: received picker update when no priority is in use (EDS returned an empty list)")
		return false
	}

	if xdsB.priorityInUse.higherThan(priority) {
		// Lower priorities should all be closed, this is an unexpected update.
		grpclog.Infof("eds: received picker update from priority lower then priorityInUse")
		return false
	}

	bState, ok := xdsB.priorityToState[priority]
	if !ok {
		bState = &balancer.State{}
		xdsB.priorityToState[priority] = bState
	}
	oldState := bState.ConnectivityState
	*bState = s

	switch s.ConnectivityState {
	case connectivity.Ready:
		return xdsB.handlePriorityWithNewStateReady(priority)
	case connectivity.TransientFailure:
		return xdsB.handlePriorityWithNewStateTransientFailure(priority)
	case connectivity.Connecting:
		return xdsB.handlePriorityWithNewStateConnecting(priority, oldState)
	default:
		// New state is Idle, should never happen. Don't forward.
		return false
	}
}

// handlePriorityWithNewStateReady handles state Ready and decides whether to
// forward update or not.
//
// An update with state Ready:
// - If it's from higher priority:
//   - Forward the update
//   - Set the priority as priorityInUse
//   - Close all priorities lower than this one
// - If it's from priorityInUse:
//   - Forward and do nothing else
//
// Caller must make sure priorityInUse is not higher than priority.
//
// Caller must hold priorityMu.
func (xdsB *EDSBalancer) handlePriorityWithNewStateReady(priority priorityType) bool {
	// If one priority higher or equal to priorityInUse goes Ready, stop the
	// init timer. If update is from higher than priorityInUse,
	// priorityInUse will be closed, and the init timer will become useless.
	if timer := xdsB.priorityInitTimer; timer != nil {
		timer.Stop()
		xdsB.priorityInitTimer = nil
	}

	if xdsB.priorityInUse.lowerThan(priority) {
		xdsB.priorityInUse = priority
		for i := priority.nextLower(); !i.lowerThan(xdsB.priorityLowest); i = i.nextLower() {
			xdsB.priorityToLocalities[i].bg.close()
		}
		return true
	}
	return true
}

// handlePriorityWithNewStateTransientFailure handles state TransientFailure and
// decides whether to forward update or not.
//
// An update with state Failure:
// - If it's from a higher priority:
//   - Do not forward, and do nothing
// - If it's from priorityInUse:
//   - If there's no lower:
//     - Forward and do nothing else
//   - If there's a lower priority:
//     - Forward
//     - Set lower as priorityInUse
//     - Start lower
//
// Caller must make sure priorityInUse is not higher than priority.
//
// Caller must hold priorityMu.
func (xdsB *EDSBalancer) handlePriorityWithNewStateTransientFailure(priority priorityType) bool {
	if xdsB.priorityInUse.lowerThan(priority) {
		return false
	}
	// priorityInUse sends a failure. Stop its init timer.
	if timer := xdsB.priorityInitTimer; timer != nil {
		timer.Stop()
		xdsB.priorityInitTimer = nil
	}
	pNext := priority.nextLower()
	if _, okNext := xdsB.priorityToLocalities[pNext]; !okNext {
		return true
	}
	xdsB.startPriority(pNext)
	return true
}

// handlePriorityWithNewStateConnecting handles state Connecting and decides
// whether to forward update or not.
//
// An update with state Connecting:
// - If it's from a higher priority
//   - Do nothing
// - If it's from priorityInUse, the behavior depends on previous state.
//
// When new state is Connecting, the behavior depends on previous state. If the
// previous state was Ready, this is a transition out from Ready to Connecting.
// Assuming there are multiple backends in the same priority, this mean we are
// in a bad situation and we should failover to the next priority (Side note:
// the current connectivity state aggregating algorhtim (e.g. round-robin) is
// not handling this right, because if many backends all go from Ready to
// Connecting, the overall situation is more like TransientFailure, not
// Connecting).
//
// If the previous state was Idle, we don't do anything special with failure,
// and simply forward the update. The init timer should be in process, will
// handle failover if it timeouts. If the previous state was TransientFailure,
// we do not forward, because the lower priority is in use.
//
// Caller must make sure priorityInUse is not higher than priority.
//
// Caller must hold priorityMu.
func (xdsB *EDSBalancer) handlePriorityWithNewStateConnecting(priority priorityType, oldState connectivity.State) bool {
	if xdsB.priorityInUse.lowerThan(priority) {
		return false
	}

	switch oldState {
	case connectivity.Ready:
		pNext := priority.nextLower()
		if _, okNext := xdsB.priorityToLocalities[pNext]; !okNext {
			return true
		}
		xdsB.startPriority(pNext)
		return true
	case connectivity.Idle:
		return true
	case connectivity.TransientFailure:
		return false
	default:
		// Old state is Connecting or Shutdown. Don't forward.
		return false
	}
}

// priorityType represents the priority from EDS response.
//
// 0 is the highest priority. The bigger the number, the lower the priority.
type priorityType struct {
	set bool
	p   uint32
}

func newPriorityType(p uint32) priorityType {
	return priorityType{
		set: true,
		p:   p,
	}
}

func newPriorityTypeUnset() priorityType {
	return priorityType{}
}

func (p priorityType) isSet() bool {
	return p.set
}

func (p priorityType) equal(p2 priorityType) bool {
	if !p.isSet() || !p2.isSet() {
		panic("priority unset")
	}
	return p == p2
}

func (p priorityType) higherThan(p2 priorityType) bool {
	if !p.isSet() || !p2.isSet() {
		panic("priority unset")
	}
	return p.p < p2.p
}

func (p priorityType) lowerThan(p2 priorityType) bool {
	if !p.isSet() || !p2.isSet() {
		panic("priority unset")
	}
	return p.p > p2.p
}

func (p priorityType) nextLower() priorityType {
	if !p.isSet() {
		panic("priority unset")
	}
	return priorityType{
		set: true,
		p:   p.p + 1,
	}
}
