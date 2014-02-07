package http

import (
	"net/http"
	"strings"
)

func NewLowerQueryParamsHandler(hdlr http.Handler) *LowerQueryParamsHandler {
	return &LowerQueryParamsHandler{hdlr}
}

type LowerQueryParamsHandler struct {
	Handler http.Handler
}

func (h *LowerQueryParamsHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err == nil {
		lowerBoolQueryParams(req)
	}
	h.Handler.ServeHTTP(w, req)
}

func lowerBoolQueryParams(req *http.Request) {
	form := req.Form
	for key, vals := range form {
		for i, val := range vals {
			lowered := strings.ToLower(val)
			if lowered == "true" || lowered == "false" {
				req.Form[key][i] = lowered
			} else {
				req.Form[key][i] = val
			}
		}
	}
}
