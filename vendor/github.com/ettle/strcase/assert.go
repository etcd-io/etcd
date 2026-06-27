package strcase

// We use a lightweight replacement for testify/assert to reduce dependencies

// testingT interface allows us to test our assert functions
type testingT interface {
	Logf(format string, args ...interface{})
	Fail()
}

// assertTrue will fail if the value is not true
func assertTrue(t testingT, value bool) {
	if !value {
		t.Fail()
	}
}

// assertEqual will fail if the two strings are not equal
func assertEqual(t testingT, expected, actual string) {
	if expected != actual {
		t.Logf("Expected: %s Actual: %s", expected, actual)
		t.Fail()
	}
}
