package printers

import (
	"encoding/json"
	"io"

	"github.com/golangci/golangci-lint/v2/pkg/report"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

// JSON prints issues in a JSON representation.
type JSON struct {
	rd *report.Data
	w  io.Writer
}

func NewJSON(w io.Writer, rd *report.Data) *JSON {
	return &JSON{
		rd: rd,
		w:  w,
	}
}

type JSONResult struct {
	Issues []*result.Issue
	Report *report.Data
}

func (p JSON) Print(issues []*result.Issue) error {
	res := JSONResult{
		Issues: issues,
		Report: p.rd,
	}
	if res.Issues == nil {
		res.Issues = []*result.Issue{}
	}

	return json.NewEncoder(p.w).Encode(res)
}
