package pqtime

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var errInvalidTimestamp = errors.New("invalid timestamp")

type timestampParser struct {
	err error
}

func (p *timestampParser) expect(str string, char byte, pos int) {
	if p.err != nil {
		return
	}
	if pos+1 > len(str) {
		p.err = errInvalidTimestamp
		return
	}
	if c := str[pos]; c != char && p.err == nil {
		p.err = fmt.Errorf("expected '%v' at position %v; got '%v'", char, pos, c)
	}
}

func (p *timestampParser) mustAtoi(str string, begin int, end int) int {
	if p.err != nil {
		return 0
	}
	if begin < 0 || end < 0 || begin > end || end > len(str) {
		p.err = errInvalidTimestamp
		return 0
	}
	result, err := strconv.Atoi(str[begin:end])
	if err != nil {
		if p.err == nil {
			p.err = fmt.Errorf("expected number; got '%v'", str)
		}
		return 0
	}
	return result
}

func Parse(currentLocation *time.Location, str string) (time.Time, error) {
	p := timestampParser{}

	monSep := strings.IndexRune(str, '-')
	// this is Gregorian year, not ISO Year
	// In Gregorian system, the year 1 BC is followed by AD 1
	year := p.mustAtoi(str, 0, monSep)
	daySep := monSep + 3
	month := p.mustAtoi(str, monSep+1, daySep)
	p.expect(str, '-', daySep)
	timeSep := daySep + 3
	day := p.mustAtoi(str, daySep+1, timeSep)

	minLen := monSep + len("01-01") + 1

	isBC := strings.HasSuffix(str, " BC")
	if isBC {
		minLen += 3
	}

	var hour, minute, second int
	if len(str) > minLen {
		p.expect(str, ' ', timeSep)
		minSep := timeSep + 3
		p.expect(str, ':', minSep)
		hour = p.mustAtoi(str, timeSep+1, minSep)
		secSep := minSep + 3
		p.expect(str, ':', secSep)
		minute = p.mustAtoi(str, minSep+1, secSep)
		secEnd := secSep + 3
		second = p.mustAtoi(str, secSep+1, secEnd)
	}
	remainderIdx := monSep + len("01-01 00:00:00") + 1
	// Three optional (but ordered) sections follow: the
	// fractional seconds, the time zone offset, and the BC
	// designation. We set them up here and adjust the other
	// offsets if the preceding sections exist.

	nanoSec := 0
	tzOff := 0

	if remainderIdx < len(str) && str[remainderIdx] == '.' {
		fracStart := remainderIdx + 1
		fracOff := strings.IndexAny(str[fracStart:], "-+Z ")
		if fracOff < 0 {
			fracOff = len(str) - fracStart
		}
		fracSec := p.mustAtoi(str, fracStart, fracStart+fracOff)
		nanoSec = fracSec * (1000000000 / int(math.Pow(10, float64(fracOff))))

		remainderIdx += fracOff + 1
	}
	if tzStart := remainderIdx; tzStart < len(str) && (str[tzStart] == '-' || str[tzStart] == '+') {
		// time zone separator is always '-' or '+' or 'Z' (UTC is +00)
		var tzSign int
		switch c := str[tzStart]; c {
		case '-':
			tzSign = -1
		case '+':
			tzSign = +1
		default:
			return time.Time{}, fmt.Errorf("expected '-' or '+' at position %v; got %v", tzStart, c)
		}
		tzHours := p.mustAtoi(str, tzStart+1, tzStart+3)
		remainderIdx += 3
		var tzMin, tzSec int
		if remainderIdx < len(str) && str[remainderIdx] == ':' {
			tzMin = p.mustAtoi(str, remainderIdx+1, remainderIdx+3)
			remainderIdx += 3
		}
		if remainderIdx < len(str) && str[remainderIdx] == ':' {
			tzSec = p.mustAtoi(str, remainderIdx+1, remainderIdx+3)
			remainderIdx += 3
		}
		tzOff = tzSign * ((tzHours * 60 * 60) + (tzMin * 60) + tzSec)
	} else if tzStart < len(str) && str[tzStart] == 'Z' {
		// time zone Z separator indicates UTC is +00
		remainderIdx += 1
	}

	var isoYear int

	if isBC {
		isoYear = 1 - year
		remainderIdx += 3
	} else {
		isoYear = year
	}
	if remainderIdx < len(str) {
		return time.Time{}, fmt.Errorf("expected end of input, got %v", str[remainderIdx:])
	}
	t := time.Date(isoYear, time.Month(month), day,
		hour, minute, second, nanoSec,
		globalLocationCache.getLocation(tzOff))

	if currentLocation != nil {
		// Set the location of the returned Time based on the session's
		// TimeZone value, but only if the local time zone database agrees with
		// the remote database on the offset.
		lt := t.In(currentLocation)
		_, newOff := lt.Zone()
		if newOff == tzOff {
			t = lt
		}
	}

	return t, p.err
}

// Format into Postgres' text format for timestamps.
func Format(t time.Time) []byte {
	// Need to send dates before 0001 A.D. with " BC" suffix, instead of the
	// minus sign preferred by Go.
	// Beware, "0000" in ISO is "1 BC", "-0001" is "2 BC" and so on
	bc := false
	if t.Year() <= 0 {
		// flip year sign, and add 1, e.g: "0" will be "1", and "-10" will be "11"
		t = t.AddDate((-t.Year())*2+1, 0, 0)
		bc = true
	}
	b := []byte(t.Format("2006-01-02 15:04:05.999999999Z07:00"))

	_, offset := t.Zone()
	offset %= 60
	if offset != 0 {
		// RFC3339Nano already printed the minus sign
		if offset < 0 {
			offset = -offset
		}

		b = append(b, ':')
		if offset < 10 {
			b = append(b, '0')
		}
		b = strconv.AppendInt(b, int64(offset), 10)
	}

	if bc {
		b = append(b, " BC"...)
	}
	return b
}
