package uuid

import (
	"strings"
)

// Format represents different styles a UUID can be printed in constants
// represent a pattern used by the package with which to print a UUID.
type Format string

// The following are the default Formats supplied by the uuid package.
const (
	FormatHex        Format = "%x%x%x%x%x"
	FormatHexCurly   Format = "{%x%x%x%x%x}"
	FormatHexBracket Format = "(%x%x%x%x%x)"

	// FormatCanonical is the default format.
	FormatCanonical Format = "%x-%x-%x-%x-%x"

	FormatCanonicalCurly   Format = "{%x-%x-%x-%x-%x}"
	FormatCanonicalBracket Format = "(%x-%x-%x-%x-%x)"
	FormatUrn              Format = "urn:uuid:" + FormatCanonical
)

var (
	printFormat = FormatCanonical
	defaultFormats map[Format]bool = make(map[Format]bool)
)

func init() {
	defaultFormats[FormatHex] = true
	defaultFormats[FormatHexCurly] = true
	defaultFormats[FormatHexBracket] = true
	defaultFormats[FormatCanonical] = true
	defaultFormats[FormatCanonicalCurly] = true
	defaultFormats[FormatCanonicalBracket] = true
	defaultFormats[FormatUrn] = true
}

// SwitchFormat switches the default printing format for ALL UUIDs.
//
// The default is canonical uuid.Format.FormatCanonical which has been
// optimised for use with this package. It is twice as fast compared to other
// formats. However, non package formats are still very quick.
//
// A valid format will have 5 groups of [%x|%X] or follow the pattern,
// *%[xX]*%[xX]*%[xX]*%[xX]*%[xX]*. If the supplied format does not meet this
// standard the function will panic. Note any extra uses of [%] outside of the
// [%x|%X] will also cause a panic.
// Constant uuid.Formats have been provided for most likely formats.
func SwitchFormat(form Format) {
	checkFormat(form)
	printFormat = form
}

// SwitchFormatToUpper is a convenience function to set the uuid.Format to uppercase
// versions.
func SwitchFormatToUpper(form Format) {
	SwitchFormat(Format(strings.ToUpper(string(form))))
}

// Formatter will return a string representation of the given UUID.
//
// Use this for one time formatting when setting the Format using
// uuid.SwitchFormat would be overkill.
//
// A valid format will have 5 groups of [%x|%X] or follow the pattern,
// *%[xX]*%[xX]*%[xX]*%[xX]*%[xX]*. If the supplied format does not meet this
// standard the function will panic. Note any extra uses of [%] outside of the
// [%x|%X] will also cause a panic.
func Formatter(id Implementation, form Format) string {
	checkFormat(form)
	return formatUuid(id.Bytes(), form)
}

func checkFormat(form Format) {
	if defaultFormats[form] {
		return
	}
	s := strings.ToLower(string(form))
	if strings.Count(s, "%x") != 5 {
		panic("uuid: invalid format")
	}
	s = strings.Replace(s, "%x", "", -1)
	if strings.Count(s, "%") > 0 {
		panic("uuid: invalid format")
	}
}

const (
	hexTable      = "0123456789abcdef"
	hexUpperTable = "0123456789ABCDEF"

	canonicalLength      = length*2 + 4
	formatArgCount       = 10
	uuidStringBufferSize = length*2 - formatArgCount
)

var groups = [...]int{4, 2, 2, 2, 6}

func formatUuid(src []byte, form Format) string {
	if form == FormatCanonical {
		return string(formatCanonical(src))
	}
	return string(format(src, string(form)))
}

func format(src []byte, form string) []byte {
	end := len(form)
	buf := make([]byte, end+uuidStringBufferSize)

	var s, ls, b, e, p int
	var u bool
	for _, v := range groups {
		ls = s
		for ; s < end && form[s] != '%'; s++ {
		}
		copy(buf[p:], form[ls:s])
		p += s - ls
		s++
		u = form[s] == 'X'
		s++
		e = b + v
		for i, t := range src[b:e] {
			j := p + i + i
			table := hexTable
			if u {
				table = hexUpperTable
			}
			buf[j] = table[t>>4]
			buf[j+1] = table[t&0x0f]
		}
		b = e
		p += v + v
	}
	ls = s
	for ; s < end && form[s] != '%'; s++ {
	}
	copy(buf[p:], form[ls:s])
	return buf
}

func formatCanonical(src []byte) []byte {
	buf := make([]byte, canonicalLength)
	var b, p, e int
	for h, v := range groups {
		e = b + v
		for i, t := range src[b:e] {
			j := p + i + i
			buf[j] = hexTable[t>>4]
			buf[j+1] = hexTable[t&0x0f]
		}
		b = e
		p += v + v
		if h < 4 {
			buf[p] = '-'
			p += 1
		}
	}
	return buf
}
