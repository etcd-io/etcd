package pkg

import "testing"

func TestMain(m *testing.M) { // want `should call os\.Exit`
	m.Run()
}
