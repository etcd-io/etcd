package pkg

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	m.Run()
	os.Exit(1)
}
