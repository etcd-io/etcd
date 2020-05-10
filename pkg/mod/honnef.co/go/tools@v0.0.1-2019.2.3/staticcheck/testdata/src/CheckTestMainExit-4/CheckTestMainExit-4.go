package pkg

import (
	"os"
	"testing"
)

func helper() { os.Exit(1) }

func TestMain(m *testing.M) { // want `should call os\.Exit`
	// FIXME(dominikh): this is a false positive
	m.Run()
	helper()
}
