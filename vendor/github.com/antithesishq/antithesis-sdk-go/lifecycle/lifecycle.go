//go:build !no_antithesis_sdk

// Package lifecycle informs the Antithesis environment that particular test phases or milestones have been reached. It is part of the [Antithesis Go SDK], which enables Go applications to integrate with the [Antithesis platform].
//
// Both functions take the parameter details: Optional additional information provided by the user to add context for assertion failures. The information that is logged will appear in the logs section of a [triage report]. Normally the values passed to details are evaluated at runtime.
//
// [Antithesis Go SDK]: https://antithesis.com/docs/using_antithesis/sdk/go/
// [Antithesis platform]: https://antithesis.com
// [triage report]: https://antithesis.com/docs/reports/triage/
package lifecycle

import (
	"github.com/antithesishq/antithesis-sdk-go/internal"
)

// SetupComplete indicates to Antithesis that setup has completed. Call this function when your system and workload are fully initialized. After this function is called, Antithesis will take a snapshot of your system and begin [injecting faults].
//
// Calling this function multiple times or from multiple processes will have no effect. Antithesis will treat the first time any process called this function as the moment that the setup was completed.
//
// [injecting faults]: https://antithesis.com/docs/applications/reliability/fault_injection/
func SetupComplete(details any) {
	statusBlock := map[string]any{
		"status":  "complete",
		"details": details,
	}
	internal.Json_data(map[string]any{"antithesis_setup": statusBlock})
}

// SendEvent indicates to Antithesis that a certain event has been reached. It provides greater information about the ordering of events during the course of testing in Antithesis.
//
// In addition to details, you also provide an eventName, which is the name of the event that you are logging. This name will appear in the logs section of a [triage report].
//
// [triage report]: https://antithesis.com/docs/reports/triage/
func SendEvent(eventName string, details any) {
	internal.Json_data(map[string]any{eventName: details})
}
