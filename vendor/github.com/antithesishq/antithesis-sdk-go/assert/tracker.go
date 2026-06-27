//go:build !no_antithesis_sdk

package assert

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/antithesishq/antithesis-sdk-go/internal"
)

type trackerInfo struct {
	Filename  string
	Classname string
	PassCount int
	FailCount int
}

type emitTracker map[string]*trackerInfo

// assert_tracker (global) keeps track of the unique asserts evaluated
var (
	assertTracker    emitTracker = make(emitTracker)
	trackerMutex     sync.Mutex
	trackerInfoMutex sync.Mutex
)

func (tracker emitTracker) getTrackerEntry(messageKey string, filename, classname string) *trackerInfo {
	var trackerEntry *trackerInfo
	var ok bool

	if tracker == nil {
		return nil
	}

	trackerMutex.Lock()
	defer trackerMutex.Unlock()
	if trackerEntry, ok = tracker[messageKey]; !ok {
		trackerEntry = newTrackerInfo(filename, classname)
		tracker[messageKey] = trackerEntry
	}
	return trackerEntry
}

func newTrackerInfo(filename, classname string) *trackerInfo {
	trackerInfo := trackerInfo{
		PassCount: 0,
		FailCount: 0,
		Filename:  filename,
		Classname: classname,
	}
	return &trackerInfo
}

func (ti *trackerInfo) emit(ai *assertInfo) {
	if ti == nil || ai == nil {
		return
	}

	// Registrations are just sent to voidstar
	hit := ai.Hit
	if !hit {
		emitAssert(ai)
		return
	}

	var err error
	cond := ai.Condition

	trackerInfoMutex.Lock()
	defer trackerInfoMutex.Unlock()
	if cond {
		if ti.PassCount == 0 {
			err = emitAssert(ai)
		}
		if err == nil {
			ti.PassCount++
		}
		return
	}
	if ti.FailCount == 0 {
		err = emitAssert(ai)
	}
	if err == nil {
		ti.FailCount++
	}
}

func versionMessage() {
	languageBlock := map[string]any{
		"name":    "Go",
		"version": runtime.Version(),
	}
	versionBlock := map[string]any{
		"language":         languageBlock,
		"sdk_version":      internal.SDK_Version,
		"protocol_version": internal.Protocol_Version,
	}
	internal.Json_data(map[string]any{"antithesis_sdk": versionBlock})
}

// package-level flag
var hasEmitted atomic.Bool // initialzed to false

func emitAssert(ai *assertInfo) error {
	if hasEmitted.CompareAndSwap(false, true) {
		versionMessage()
	}
	return internal.Json_data(wrappedAssertInfo{ai})
}
