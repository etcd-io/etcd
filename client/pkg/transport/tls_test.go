package transport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateSecureEndpoints(t *testing.T) {
	tlsInfo, err := createSelfCert(t)
	if err != nil {
		t.Fatalf("unable to create cert: %v", err)
	}

	remoteAddr := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.RemoteAddr))
	}
	srv := httptest.NewServer(http.HandlerFunc(remoteAddr))
	defer srv.Close()

	insecureEps := []string{
		"http://" + srv.Listener.Addr().String(),
		"invalid remote address",
	}
	if _, err := ValidateSecureEndpoints(*tlsInfo, insecureEps); err == nil || !strings.Contains(err.Error(), "is insecure") {
		t.Error("validate secure endpoints should fail")
	}

	secureEps := []string{
		"https://" + srv.Listener.Addr().String(),
	}
	if _, err := ValidateSecureEndpoints(*tlsInfo, secureEps); err != nil {
		t.Error("validate secure endpoints should succeed")
	}
}
