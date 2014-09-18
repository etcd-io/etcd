package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadonlyHandler(t *testing.T) {
	fixture := func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	hdlrFunc := readonlyHandlerFunc(http.HandlerFunc(fixture))

	tests := []struct {
		method string
		want   int
	}{
		// GET is only passing method
		{"GET", http.StatusOK},

		// everything but GET is StatusNotImplemented
		{"POST", http.StatusNotImplemented},
		{"PUT", http.StatusNotImplemented},
		{"PATCH", http.StatusNotImplemented},
		{"DELETE", http.StatusNotImplemented},
		{"FOO", http.StatusNotImplemented},
	}

	for i, tt := range tests {
		req, _ := http.NewRequest(tt.method, "http://example.com", nil)
		rr := httptest.NewRecorder()
		hdlrFunc(rr, req)

		if tt.want != rr.Code {
			t.Errorf("#%d: incorrect HTTP status code: method=%s want=%d got=%d", i, tt.method, tt.want, rr.Code)
		}
	}
}
