package verify

import (
	"os"
	"testing"
)

// RED: disabling verifications when the env var was unset must leave it
// unset, not set to an empty string.
func TestDisableVerificationsRestoresUnset(t *testing.T) {
	os.Unsetenv(envVerify)
	restore := DisableVerifications()
	restore()
	if _, ok := os.LookupEnv(envVerify); ok {
		t.Fatalf("%s should be unset after restore, got %q", envVerify, os.Getenv(envVerify))
	}
}

// RED: enabling then restoring must also preserve the unset state.
func TestEnableVerificationsRestoresUnset(t *testing.T) {
	os.Unsetenv(envVerify)
	restore := EnableVerifications(envVerifyValueAssert)
	restore()
	if _, ok := os.LookupEnv(envVerify); ok {
		t.Fatalf("%s should be unset after restore, got %q", envVerify, os.Getenv(envVerify))
	}
}
