//go:build !no_antithesis_sdk

package assert

import (
	"sync"

	"github.com/antithesishq/antithesis-sdk-go/internal"
)

// TODO: Tracker is intended to prevent sending the same guidance
// more than once.  In this case, we always send, so the tracker
// is not presently used.
type booleanGuidance struct {
	n int
}

type booleanGuidanceTracker map[string]*booleanGuidance

var (
	boolean_guidance_tracker       booleanGuidanceTracker = make(booleanGuidanceTracker)
	boolean_guidance_tracker_mutex sync.Mutex
	boolean_guidance_info_mutex    sync.Mutex
)

func (tracker booleanGuidanceTracker) getTrackerEntry(messageKey string) *booleanGuidance {
	var trackerEntry *booleanGuidance
	var ok bool

	if tracker == nil {
		return nil
	}

	boolean_guidance_tracker_mutex.Lock()
	defer boolean_guidance_tracker_mutex.Unlock()
	if trackerEntry, ok = boolean_guidance_tracker[messageKey]; !ok {
		trackerEntry = newBooleanGuidance()
		tracker[messageKey] = trackerEntry
	}

	return trackerEntry
}

// Create a boolean guidance tracker
func newBooleanGuidance() *booleanGuidance {
	trackerInfo := booleanGuidance{}
	return &trackerInfo
}

func (tI *booleanGuidance) send_value(bgI *booleanGuidanceInfo) {
	if tI == nil {
		return
	}

	boolean_guidance_info_mutex.Lock()
	defer boolean_guidance_info_mutex.Unlock()

	// The tracker entry should be consulted to determine
	// if this Guidance info has already been sent, or not.

	emitBooleanGuidance(bgI)
}

func emitBooleanGuidance(bgI *booleanGuidanceInfo) error {
	return internal.Json_data(map[string]any{"antithesis_guidance": bgI})
}
