package parser

import "fmt"

// Value parses a tag value.
// The value is split on comma, and escaped commas are ignored.
func Value(raw string, escapeComma bool) ([]string, error) {
	if raw == "" {
		return []string{""}, nil
	}

	var values []string

	for raw != "" {
		i := indexEscaped(raw, 0, escapeComma)

		if i == 0 {
			values = append(values, "")
			raw = raw[i+1:]

			if raw == "" {
				values = append(values, "")

				break
			}

			continue
		}

		if i == len(raw) {
			values = append(values, raw[:i])

			break
		}

		if i > len(raw) {
			return nil, fmt.Errorf("syntax error in struct tag value %q", raw)
		}

		value := raw[:i]

		endComma := raw[i] == ',' && raw[i+1:] == ""

		raw = raw[i+1:]

		values = append(values, value)

		if endComma {
			values = append(values, "")
		}
	}

	return values, nil
}

func indexEscaped(raw string, i int, escapeComma bool) int {
	for i < len(raw) && raw[i] != ',' {
		i++
	}

	if !escapeComma {
		return i
	}

	if i-1 >= 0 && raw[i-1] == '\\' {
		j := i - 1

		count := 1

		for j >= 0 && raw[j] == '\\' {
			count++
			j--
		}

		if count%2 == 0 {
			i++

			return indexEscaped(raw, i, escapeComma)
		}
	}

	return i
}
