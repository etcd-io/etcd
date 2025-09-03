package engine

import (
	"encoding/json"

	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine/intf"
)

type report struct {
	Input   any   `json:"input,omitempty"`
	Results []any `json:"results,omitempty"`
}

// Diagnose runs all provided plugins and returns a JSON report.
// It logs plugin progress and individual results to stderr.
func Diagnose(input any, plugins []intf.Plugin) ([]byte, error) {
	rp := report{
		Input: input,
	}
	for _, plugin := range plugins {
		result := plugin.Diagnose()
		rp.Results = append(rp.Results, result)
	}

	return json.MarshalIndent(rp, "", "\t")
}
