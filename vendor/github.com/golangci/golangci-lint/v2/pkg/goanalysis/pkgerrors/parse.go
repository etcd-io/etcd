package pkgerrors

import (
	"errors"
	"fmt"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/pkg/result"
)

func parseError(srcErr packages.Error) (*result.Issue, error) {
	pos, err := parseErrorPosition(srcErr.Pos)
	if err != nil {
		return nil, err
	}

	return &result.Issue{
		Pos:        *pos,
		Text:       srcErr.Msg,
		FromLinter: "typecheck",
	}, nil
}

func parseErrorPosition(pos string) (*token.Position, error) {
	// file:line(<optional>:column)
	parts := strings.Split(pos, ":")
	if len(parts) == 1 {
		return nil, errors.New("no colons")
	}

	file := parts[0]
	line, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("can't parse line number %q: %w", parts[1], err)
	}

	var column int
	if len(parts) == 3 { // got column
		column, err = strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("failed to parse column from %q: %w", parts[2], err)
		}
	}

	return &token.Position{
		Filename: file,
		Line:     line,
		Column:   column,
	}, nil
}
