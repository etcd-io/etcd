package shorten

import (
	"bytes"
	"fmt"
	"go/format"
	"log/slog"
	"os"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/golangci/golines/shorten/internal/comments"
	"github.com/golangci/golines/shorten/internal/graph"
	"github.com/golangci/golines/shorten/internal/tags"
)

// The maximum number of shortening "rounds" that we'll allow.
// The shortening process should converge quickly,
// but we have this here as a safety mechanism to prevent loops that prevent termination.
const maxRounds = 20

// Config stores the configuration options exposed by a Shortener instance.
type Config struct {
	// MaxLen Max target width for each line
	MaxLen int

	// TabLen Width of a tab character
	TabLen int

	// KeepAnnotations Whether to keep annotations in the final result (for debugging only)
	KeepAnnotations bool

	// ShortenComments Whether to shorten comments
	ShortenComments bool

	// ReformatTags Whether to reformat struct tags in addition to shortening long lines
	ReformatTags bool

	// DotFile Path to write dot-formatted output to (for debugging only)
	DotFile string

	// ChainSplitDots Whether to split chain methods by putting dots at the ends of lines
	ChainSplitDots bool
}

// NewDefaultConfig returns a [Config] with default values.
func NewDefaultConfig() *Config {
	return &Config{
		MaxLen:          100,
		TabLen:          4,
		KeepAnnotations: false,
		ShortenComments: false,
		ReformatTags:    true,
		DotFile:         "",
		ChainSplitDots:  true,
	}
}

// Options is the type for configuring options of a [Shortener] instance.
type Options func(*Shortener)

// WithLogger sets the logger to use it for a [Shortener] instance.
func WithLogger(logger Logger) Options {
	return func(s *Shortener) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// Shortener shortens a single go file according to a small set of user style preferences.
type Shortener struct {
	config *Config

	commentsShortener *comments.Shortener

	logger Logger
}

// NewShortener creates a new shortener instance from the provided config.
func NewShortener(config *Config, opts ...Options) *Shortener {
	if config == nil {
		config = NewDefaultConfig()
	}

	s := &Shortener{
		config: config,
		logger: &noopLogger{},
	}

	if config.ShortenComments {
		s.commentsShortener = &comments.Shortener{
			MaxLen: config.MaxLen,
			TabLen: config.TabLen,
		}
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Process shortens the provided golang file content bytes.
func (s *Shortener) Process(content []byte) ([]byte, error) {
	var round int

	var err error

	// Do initial, non-line-length-aware formatting
	content, err = format.Source(content)
	if err != nil {
		return nil, fmt.Errorf("error formatting source: %w", err)
	}

	for {
		s.logger.Debug("starting round", slog.Int("round", round))

		// Annotate all long lines
		lines := strings.Split(string(content), "\n")
		annotatedLines, nbLinesToShorten := s.annotateLongLines(lines)

		if !s.shouldContinue(nbLinesToShorten, round, lines) {
			s.logger.Debug("nothing more to shorten or reformat, stopping")

			break
		}

		content = []byte(strings.Join(annotatedLines, "\n"))

		// Generate AST
		result, err := decorator.Parse(content)
		if err != nil {
			return nil, err
		}

		if s.config.DotFile != "" {
			err = s.createDot(result)
			if err != nil {
				return nil, err
			}
		}

		// Process the file.
		s.formatFile(result)

		// Materialize output
		output := bytes.NewBuffer([]byte{})

		err = decorator.Fprint(output, result)
		if err != nil {
			return nil, fmt.Errorf("error parsing source: %w", err)
		}

		content = output.Bytes()

		round++

		if round > maxRounds {
			s.logger.Debug("hit max rounds, stopping")

			break
		}
	}

	if !s.config.KeepAnnotations {
		content = removeAnnotations(content)
	}

	if s.commentsShortener != nil {
		content = s.commentsShortener.Process(content)
	}

	// Do the final round of non-line-length-aware formatting after we've fixed up the comments
	content, err = format.Source(content)
	if err != nil {
		return nil, fmt.Errorf("error formatting source: %w", err)
	}

	return content, nil
}

// shouldContinue returns true:
// if there are lines to shorten,
// or if this is the first round (0),
// and the option to reformat struct tags is enabled,
// and there are struct tags with multiple entries.
func (s *Shortener) shouldContinue(nbLinesToShorten, round int, lines []string) bool {
	return nbLinesToShorten > 0 ||
		round == 0 && s.config.ReformatTags && tags.HasMultipleEntries(lines)
}

func (s *Shortener) createDot(result dst.Node) error {
	dotFile, err := os.Create(s.config.DotFile)
	if err != nil {
		return err
	}

	defer dotFile.Close()

	s.logger.Debug("writing dot file output", slog.String("file", s.config.DotFile))

	return graph.CreateDot(result, dotFile)
}
