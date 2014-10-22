package command

import (
	"errors"
	"io"
	"io/ioutil"
	"strings"
)

var (
	ErrNoAvailSrc = errors.New("no available argument and stdin")
)

// trimsplit slices s into all substrings separated by sep and returns a
// slice of the substrings between the separator with all leading and trailing
// white space removed, as defined by Unicode.
func trimsplit(s, sep string) []string {
	raw := strings.Split(s, ",")
	trimmed := make([]string, 0)
	for _, r := range raw {
		trimmed = append(trimmed, strings.TrimSpace(r))
	}
	return trimmed
}

func argOrStdin(args []string, stdin io.Reader, i int) (string, error) {
	if i < len(args) {
		return args[i], nil
	}
	bytes, err := ioutil.ReadAll(stdin)
	if string(bytes) == "" || err != nil {
		return "", ErrNoAvailSrc
	}
	return string(bytes), nil
}
