package tags

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dave/dst"
	"github.com/golangci/golines/shorten/internal/annotation"
	"github.com/ldez/structtags/parser"
)

var structTagRegexp = regexp.MustCompile("`([ ]*[a-zA-Z0-9_-]+:\".*\"[ ]*){2,}`")

// HasMultipleEntries returns whether the given lines have a multi-entries struct line.
// It's used as an optimization step to avoid unnecessary shortening rounds.
func HasMultipleEntries(lines []string) bool {
	return slices.ContainsFunc(lines, structTagRegexp.MatchString)
}

// FormatStructTags formats struct tags so that the keys within each block of fields are aligned.
// It's not technically a shortening (and it usually makes these tags longer), so it's being
// kept separate from the core shortening logic for now.
//
// See the struct_tags fixture for examples.
func FormatStructTags(fieldList *dst.FieldList) {
	if fieldList == nil || len(fieldList.List) == 0 {
		return
	}

	var blockFields []*dst.Field

	// Divide fields into "blocks" so that we don't do alignments across blank lines and comments.
	for _, field := range fieldList.List {
		if isEndFieldsBlock(field) {
			align(blockFields)

			blockFields = blockFields[:0]
		}

		blockFields = append(blockFields, field)
	}

	align(blockFields)
}

func isEndFieldsBlock(field *dst.Field) bool {
	return field.Decorations().Before == dst.EmptyLine ||
		slices.ContainsFunc(field.Decorations().Start.All(), func(s string) bool {
			return !annotation.Is(s)
		})
}

// align formats the struct tags within a single field block.
// NOTE(ldez): all the code of `align` is not related to shorten,
// instead of shortening a line, it increases the line length.
// It must be either:
// - disabled by default
// - or the additional spaces should be an option
// - or maybe removed.
func align(fields []*dst.Field) {
	if len(fields) == 0 {
		return
	}

	maxTagWidths := map[string]int{}

	var tagKeys []string

	tagKVs := make([]map[string]string, len(fields))

	var (
		maxTypeWidth  int
		invalidWidths bool
	)

	// First, scan over all field tags so that we can understand their values and widths

	for f, field := range fields {
		if len(field.Names) > 0 {
			typeWidth, err := getWidth(field.Type)
			if err != nil {
				// We couldn't figure out the proper width of this field
				invalidWidths = true
			} else if typeWidth > maxTypeWidth {
				maxTypeWidth = typeWidth
			}
		}

		if field.Tag == nil {
			continue
		}

		tagValue := field.Tag.Value

		// The dst library doesn't strip off the backticks, so we need to do this manually
		if tagValue[0] != '`' || tagValue[len(tagValue)-1] != '`' {
			continue
		}

		tagValue = tagValue[1 : len(tagValue)-1]

		entries, err := parser.Tag(tagValue, newFiller())
		if err != nil {
			return
		}

		for _, entry := range entries {
			// Tag is key, value, and some extra chars (two quotes + one colon)
			width := utf8.RuneCountInString(entry.Content)

			if _, ok := maxTagWidths[entry.Key]; !ok {
				maxTagWidths[entry.Key] = width
				tagKeys = append(tagKeys, entry.Key)
			} else if width > maxTagWidths[entry.Key] {
				maxTagWidths[entry.Key] = width
			}

			if tagKVs[f] == nil {
				tagKVs[f] = map[string]string{}
			}

			tagKVs[f][entry.Key] = entry.Content
		}
	}

	// Go over all the fields again, replacing each tag with a reformatted one
	for f, field := range fields {
		if tagKVs[f] == nil {
			continue
		}

		var tagComponents []string

		if len(field.Names) == 0 && maxTypeWidth > 0 && !invalidWidths {
			// Add extra spacing at beginning so that tag aligns with named field tags
			tagComponents = append(tagComponents, "")

			tagComponents[len(tagComponents)-1] += strings.Repeat(" ", maxTypeWidth)
		}

		for _, key := range tagKeys {
			content, ok := tagKVs[f][key]
			lenUsed := 0

			if ok {
				tagComponents = append(tagComponents, content)
				lenUsed += utf8.RuneCountInString(content)
			} else {
				tagComponents = append(tagComponents, "")
			}

			if len(field.Names) > 0 || !invalidWidths {
				lenRemaining := maxTagWidths[key] - lenUsed

				tagComponents[len(tagComponents)-1] += strings.Repeat(" ", lenRemaining)
			}
		}

		field.Tag.Value = fmt.Sprintf("`%s`",
			strings.TrimRight(strings.Join(tagComponents, " "), " "))
	}
}

// getWidth tries to guess the formatted width of a dst node expression.
// If this isn't (yet) possible, it returns an error.
func getWidth(node dst.Node) (int, error) {
	switch n := node.(type) {
	case *dst.ArrayType:
		eltWidth, err := getWidth(n.Elt)
		if err != nil {
			return 0, err
		}

		return 2 + eltWidth, nil

	case *dst.ChanType:
		valWidth, err := getWidth(n.Value)
		if err != nil {
			return 0, err
		}

		isSend := n.Dir&dst.SEND > 0
		isRecv := n.Dir&dst.RECV > 0

		if isSend && isRecv {
			// Channel does not include an arrow
			return 5 + valWidth, nil
		}

		// Channel includes an arrow
		return 7 + valWidth, nil

	case *dst.Ident:
		return len(n.Name), nil

	case *dst.MapType:
		keyWidth, err := getWidth(n.Key)
		if err != nil {
			return 0, err
		}

		valWidth, err := getWidth(n.Value)
		if err != nil {
			return 0, err
		}

		return 5 + keyWidth + valWidth, nil

	case *dst.StarExpr:
		xWidth, err := getWidth(n.X)
		if err != nil {
			return 0, err
		}

		return 1 + xWidth, nil
	}

	return 0, fmt.Errorf("could not get the width of node %+v", node)
}
