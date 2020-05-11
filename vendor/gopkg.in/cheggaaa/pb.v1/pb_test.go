package pb

import (
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/mattn/go-colorable"
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

func Test_Format(t *testing.T) {
	bar := New(5000).Format(strings.Join([]string{
		color.GreenString("["),
		color.New(color.BgGreen).SprintFunc()("o"),
		color.New(color.BgHiGreen).SprintFunc()("o"),
		color.New(color.BgRed).SprintFunc()("o"),
		color.GreenString("]"),
	}, "\x00"))
	w := colorable.NewColorableStdout()
	bar.Callback = func(out string) {
		w.Write([]byte(out))
	}
	bar.Add(2000)
	bar.Finish()
	bar.Finish()
}
