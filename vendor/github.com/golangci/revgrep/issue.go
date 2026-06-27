package revgrep

// Issue contains metadata about an issue found.
type Issue struct {
	// File is the name of the file as it appeared from the patch.
	File string
	// LineNo is the line number of the file.
	LineNo int
	// ColNo is the column number or 0 if none could be parsed.
	ColNo int
	// HunkPos is position from file's first @@, for new files this will be the line number.
	// See also: https://developer.github.com/v3/pulls/comments/#create-a-comment
	HunkPos int
	// Issue text as it appeared from the tool.
	Issue string
	// Message is the issue without file name, line number and column number.
	Message string
}

// InputIssue represents issue found by some linter.
type InputIssue interface {
	FilePath() string
	Line() int
}

type simpleInputIssue struct {
	filePath   string
	lineNumber int
}

func (i simpleInputIssue) FilePath() string {
	return i.filePath
}

func (i simpleInputIssue) Line() int {
	return i.lineNumber
}
