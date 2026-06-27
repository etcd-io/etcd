//go:build windows
// +build windows

package pb

import (
	"fmt"
	"log"
	"strings"

	"github.com/cheggaaa/pb/v3/termutil"
)

func (p *Pool) print(first bool) bool {
	p.m.Lock()
	defer p.m.Unlock()
	var out string
	if !first {
		coords, err := termutil.GetCursorPos()
		if err != nil {
			log.Panic(err)
		}
		coords.Y -= int16(p.lastBarsCount)
		if coords.Y < 0 {
			coords.Y = 0
		}
		coords.X = 0

		err = termutil.SetCursorPos(coords)
		if err != nil {
			log.Panic(err)
		}
	}
	cols, err := termutil.TerminalWidth()
	if err != nil {
		cols = defaultBarWidth
	}
	isFinished := true
	for _, bar := range p.bars {
		if !bar.IsFinished() {
			isFinished = false
		}
		result := bar.String()
		if r := cols - CellCount(result); r > 0 {
			result += strings.Repeat(" ", r)
		}
		out += fmt.Sprintf("\r%s\n", result)
	}
	if p.Output != nil {
		fmt.Fprint(p.Output, out)
	} else {
		fmt.Print(out)
	}
	p.lastBarsCount = len(p.bars)
	return isFinished
}
