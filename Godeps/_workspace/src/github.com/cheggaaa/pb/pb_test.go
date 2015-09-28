package pb

import (
	"testing"
)

func Test_IncrementAddsOne(t *testing.T) {
	count := 5000
	bar := New(count)
	expected := 1
	actual := bar.Increment()

	if actual != expected {
		t.Errorf("Expected {%d} was {%d}", expected, actual)
	}
}

func Test_Width(t *testing.T) {
	count := 5000
	bar := New(count)
	width := 100
	bar.SetWidth(100).Callback = func(out string) {
		if len(out) != width {
			t.Errorf("Bar width expected {%d} was {%d}", len(out), width)
		}
	}
	bar.Start()
	bar.Increment()
	bar.Finish()
}

func Test_MultipleFinish(t *testing.T) {
	bar := New(5000)
	bar.Add(2000)
	bar.Finish()
	bar.Finish()
}
