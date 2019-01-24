package mysql

import (
	"bytes"
	"strconv"
	"strings"
	"unicode"

	"github.com/pingcap/errors"
)

func formatENUS(number string, precision string) (string, error) {
	var buffer bytes.Buffer
	if unicode.IsDigit(rune(precision[0])) {
		for i, v := range precision {
			if unicode.IsDigit(v) {
				continue
			}
			precision = precision[:i]
			break
		}
	} else {
		precision = "0"
	}
	if number[0] == '-' && number[1] == '.' {
		number = strings.Replace(number, "-", "-0", 1)
	} else if number[0] == '.' {
		number = strings.Replace(number, ".", "0.", 1)
	}

	if (number[:1] == "-" && !unicode.IsDigit(rune(number[1]))) ||
		(!unicode.IsDigit(rune(number[0])) && number[:1] != "-") {
		buffer.Write([]byte{'0'})
		position, err := strconv.ParseUint(precision, 10, 64)
		if err == nil && position > 0 {
			buffer.Write([]byte{'.'})
			buffer.WriteString(strings.Repeat("0", int(position)))
		}
		return buffer.String(), nil
	} else if number[:1] == "-" {
		buffer.Write([]byte{'-'})
		number = number[1:]
	}

	for i, v := range number {
		if unicode.IsDigit(v) {
			continue
		} else if i == 1 && number[1] == '.' {
			continue
		} else if v == '.' && number[1] != '.' {
			continue
		} else {
			number = number[:i]
			break
		}
	}

	comma := []byte{','}
	parts := strings.Split(number, ".")
	pos := 0
	if len(parts[0])%3 != 0 {
		pos += len(parts[0]) % 3
		buffer.WriteString(parts[0][:pos])
		buffer.Write(comma)
	}
	for ; pos < len(parts[0]); pos += 3 {
		buffer.WriteString(parts[0][pos : pos+3])
		buffer.Write(comma)
	}
	buffer.Truncate(buffer.Len() - 1)

	position, err := strconv.ParseUint(precision, 10, 64)
	if err == nil {
		if position > 0 {
			buffer.Write([]byte{'.'})
			if len(parts) == 2 {
				if uint64(len(parts[1])) >= position {
					buffer.WriteString(parts[1][:position])
				} else {
					buffer.WriteString(parts[1])
					buffer.WriteString(strings.Repeat("0", int(position)-len(parts[1])))
				}
			} else {
				buffer.WriteString(strings.Repeat("0", int(position)))
			}
		}
	}

	return buffer.String(), nil
}

func formatZHCN(number string, precision string) (string, error) {
	return "", errors.New("not implemented")
}

func formatNotSupport(number string, precision string) (string, error) {
	return "", errors.New("not support for the specific locale")
}
