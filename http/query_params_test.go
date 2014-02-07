package http

import (
	"net/http"
	"testing"
)

type NilResponseWriter struct{}

func (w NilResponseWriter) Header() http.Header {
	return http.Header{}
}

func (w NilResponseWriter) Write(data []byte) (int, error) {
	return 0, nil
}

func (w NilResponseWriter) WriteHeader(code int) {
	return
}

type FunctionHandler struct {
	f func(*http.Request)
}

func (h FunctionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.f(r)
}

func TestQueryParamsLowered(t *testing.T) {
	assertFunc := func(req *http.Request) {
		if len(req.Form["One"]) != 1 || req.Form["One"][0] != "true" {
			t.Errorf("Unexpected value for One: %s", req.Form["One"])
		} else if len(req.Form["TWO"]) != 1 || req.Form["TWO"][0] != "false" {
			t.Errorf("Unexpected value for TWO")
		} else if len(req.Form["three"]) != 2 || req.Form["three"][0] != "true" || req.Form["three"][1] != "false" {
			t.Errorf("Unexpected value for three")
		}
	}
	assertHdlr := FunctionHandler{assertFunc}
	hdlr := NewLowerQueryParamsHandler(assertHdlr)
	respWriter := NilResponseWriter{}

	req, _ := http.NewRequest("GET", "http://example.com?One=TRUE&TWO=False&three=true&three=FALSE", nil)
	hdlr.ServeHTTP(respWriter, req)
}
