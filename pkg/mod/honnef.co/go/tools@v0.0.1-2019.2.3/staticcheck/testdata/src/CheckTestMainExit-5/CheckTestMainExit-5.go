package pkg

import (
	"os"
	"testing"
)

func helper(m *testing.M) { os.Exit(m.Run()) }

func TestMain(m *testing.M) {
	helper(m)
}
