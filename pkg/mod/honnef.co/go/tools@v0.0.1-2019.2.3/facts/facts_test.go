package facts

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestDeprecated(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Deprecated, "Deprecated")
}

func TestPurity(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Purity, "Purity")
}
