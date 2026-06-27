package section

import (
	"errors"
	"fmt"

	"github.com/daixiang0/gci/pkg/parse"
	"github.com/daixiang0/gci/pkg/utils"
)

type SectionParsingError struct {
	error
}

func (s SectionParsingError) Unwrap() error {
	return s.error
}

func (s SectionParsingError) Wrap(sectionStr string) error {
	return fmt.Errorf("failed to parse section %q: %w", sectionStr, s)
}

func (s SectionParsingError) Is(err error) bool {
	_, ok := err.(SectionParsingError)
	return ok
}

var MissingParameterClosingBracketsError = fmt.Errorf("section parameter is missing closing %q", utils.RightParenthesis)

var MoreThanOneOpeningQuotesError = fmt.Errorf("found more than one %q parameter start sequences", utils.RightParenthesis)

var SectionTypeDoesNotAcceptParametersError = errors.New("section type does not accept a parameter")

var SectionTypeDoesNotAcceptPrefixError = errors.New("section may not contain a Prefix")

var SectionTypeDoesNotAcceptSuffixError = errors.New("section may not contain a Suffix")

type EqualSpecificityMatchError struct {
	Imports            *parse.GciImports
	SectionA, SectionB Section
}

func (e EqualSpecificityMatchError) Error() string {
	return fmt.Sprintf("Import %v matched section %s and %s equally", e.Imports, e.SectionA, e.SectionB)
}

func (e EqualSpecificityMatchError) Is(err error) bool {
	_, ok := err.(EqualSpecificityMatchError)
	return ok
}

type NoMatchingSectionForImportError struct {
	Imports *parse.GciImports
}

func (n NoMatchingSectionForImportError) Error() string {
	return fmt.Sprintf("No section found for Import: %v", n.Imports)
}

func (n NoMatchingSectionForImportError) Is(err error) bool {
	_, ok := err.(NoMatchingSectionForImportError)
	return ok
}

type InvalidImportSplitError struct {
	segments []string
}

func (i InvalidImportSplitError) Error() string {
	return fmt.Sprintf("separating the inline comment from the import yielded an invalid number of segments: %v", i.segments)
}

func (i InvalidImportSplitError) Is(err error) bool {
	_, ok := err.(InvalidImportSplitError)
	return ok
}

type InvalidAliasSplitError struct {
	segments []string
}

func (i InvalidAliasSplitError) Error() string {
	return fmt.Sprintf("separating the alias from the path yielded an invalid number of segments: %v", i.segments)
}

func (i InvalidAliasSplitError) Is(err error) bool {
	_, ok := err.(InvalidAliasSplitError)
	return ok
}

var (
	MissingImportStatementError   = FileParsingError{errors.New("no import statement present in File")}
	ImportStatementNotClosedError = FileParsingError{errors.New("import statement not closed")}
)

type FileParsingError struct {
	error
}

func (f FileParsingError) Unwrap() error {
	return f.error
}

func (f FileParsingError) Is(err error) bool {
	_, ok := err.(FileParsingError)
	return ok
}
