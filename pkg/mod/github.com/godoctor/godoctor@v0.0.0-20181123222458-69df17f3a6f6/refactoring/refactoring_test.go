package refactoring_test

import (
	"testing"

	"github.com/godoctor/godoctor/engine"
	"github.com/godoctor/godoctor/refactoring/testutil"
)

const directory = "testdata/"

func TestRefactorings(t *testing.T) {
	engine.AddDefaultRefactorings()
	testutil.TestRefactorings(directory, t)
}
