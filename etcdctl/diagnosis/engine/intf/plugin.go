package intf

type Plugin interface {
	// Name returns the name of the plugin
	Name() string
	// Diagnose performs diagnosis and returns the result. If it fails
	// to do the diagnosis for any reason, it gets the detailed reason
	// included in the diagnosis result.
	Diagnose() any
}

// FailedResult is the result returned by a plugin if it fails to
// perform the diagnosis for any reason.
type FailedResult struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}
