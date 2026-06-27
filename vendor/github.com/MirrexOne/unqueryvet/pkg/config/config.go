// Package config provides configuration structures for Unqueryvet analyzer.
package config

// CustomRule represents a user-defined DSL rule for SQL analysis.
type CustomRule struct {
	// ID is a unique identifier for the rule.
	ID string `mapstructure:"id" json:"id" yaml:"id"`

	// Pattern is the SQL or code pattern to match.
	// Supports metavariables like $TABLE, $VAR, etc.
	Pattern string `mapstructure:"pattern" json:"pattern" yaml:"pattern"`

	// Patterns allows multiple patterns for a single rule.
	Patterns []string `mapstructure:"patterns" json:"patterns" yaml:"patterns,omitempty"`

	// When is an optional condition expression (evaluated with expr-lang).
	// Available variables: file, package, function, query, table, in_loop, etc.
	When string `mapstructure:"when" json:"when" yaml:"when,omitempty"`

	// Message is the diagnostic message shown when the rule triggers.
	Message string `mapstructure:"message" json:"message" yaml:"message,omitempty"`

	// Severity is the severity level (error, warning, info, ignore).
	Severity string `mapstructure:"severity" json:"severity" yaml:"severity,omitempty"`

	// Action determines what to do when the pattern matches (report, allow, ignore).
	Action string `mapstructure:"action" json:"action" yaml:"action,omitempty"`

	// Fix is an optional suggested fix message.
	Fix string `mapstructure:"fix" json:"fix" yaml:"fix,omitempty"`
}

// RuleSeverity maps built-in rule IDs to their severity levels.
type RuleSeverity map[string]string

// UnqueryvetSettings holds the configuration for the Unqueryvet analyzer.
type UnqueryvetSettings struct {
	// CheckSQLBuilders enables checking SQL builders like Squirrel for SELECT * usage
	CheckSQLBuilders bool `mapstructure:"check-sql-builders" json:"check-sql-builders" yaml:"check-sql-builders"`

	// AllowedPatterns is a list of regex patterns that are allowed to use SELECT *
	// Example: ["SELECT \\* FROM temp_.*", "SELECT \\* FROM .*_backup"]
	AllowedPatterns []string `mapstructure:"allowed-patterns" json:"allowed-patterns" yaml:"allowed-patterns"`

	// IgnoredFunctions is a list of function name patterns to ignore
	// Example: ["debug.Query", "test.*", "mock.*"]
	IgnoredFunctions []string `mapstructure:"ignored-functions" json:"ignored-functions" yaml:"ignored-functions"`

	// IgnoredFiles is a list of glob patterns for files to ignore
	// Example: ["*_test.go", "testdata/**", "mock_*.go"]
	IgnoredFiles []string `mapstructure:"ignored-files" json:"ignored-files" yaml:"ignored-files"`

	// Severity defines the diagnostic severity: "error" or "warning" (default: "warning")
	Severity string `mapstructure:"severity" json:"severity" yaml:"severity"`

	// CheckAliasedWildcard enables detection of SELECT alias.* patterns (e.g., SELECT t.* FROM users t)
	CheckAliasedWildcard bool `mapstructure:"check-aliased-wildcard" json:"check-aliased-wildcard" yaml:"check-aliased-wildcard"`

	// CheckStringConcat enables detection of SELECT * in string concatenation (e.g., "SELECT * " + "FROM users")
	CheckStringConcat bool `mapstructure:"check-string-concat" json:"check-string-concat" yaml:"check-string-concat"`

	// CheckFormatStrings enables detection of SELECT * in format functions (e.g., fmt.Sprintf)
	CheckFormatStrings bool `mapstructure:"check-format-strings" json:"check-format-strings" yaml:"check-format-strings"`

	// CheckStringBuilder enables detection of SELECT * in strings.Builder usage
	CheckStringBuilder bool `mapstructure:"check-string-builder" json:"check-string-builder" yaml:"check-string-builder"`

	// CheckSubqueries enables detection of SELECT * in subqueries (e.g., SELECT * FROM (SELECT * FROM ...))
	CheckSubqueries bool `mapstructure:"check-subqueries" json:"check-subqueries" yaml:"check-subqueries"`

	// SQLBuilders defines which SQL builder libraries to check
	SQLBuilders SQLBuildersConfig `mapstructure:"sql-builders" json:"sql-builders" yaml:"sql-builders"`

	// Rules is a map of built-in rule IDs to their severity (error, warning, info, ignore).
	// Example: {"select-star": "error", "n1-queries": "warning"}
	Rules RuleSeverity `mapstructure:"rules" json:"rules" yaml:"rules,omitempty"`

	// CustomRules is a list of user-defined DSL rules.
	CustomRules []CustomRule `mapstructure:"custom-rules" json:"custom-rules" yaml:"custom-rules,omitempty"`

	// Allow is a list of SQL patterns to allow (whitelist).
	// These patterns will not trigger any warnings.
	Allow []string `mapstructure:"allow" json:"allow" yaml:"allow,omitempty"`

	// Ignore is a list of file patterns to ignore (in addition to IgnoredFiles).
	Ignore []string `mapstructure:"ignore" json:"ignore" yaml:"ignore,omitempty"`

	// N1DetectionEnabled global flag for N+1 detection
	N1DetectionEnabled bool `mapstructure:"check-n1-queries" json:"check-n1-queries" yaml:"check-n1-queries,omitempty"`

	// SQLInjectionDetectionEnabled global flag for SQL injection detection
	SQLInjectionDetectionEnabled bool `mapstructure:"check-sql-injection" json:"check-sql-injection" yaml:"check-sql-injection,omitempty"`

	// TxLeakDetectionEnabled global flag for unclosed transaction detection
	TxLeakDetectionEnabled bool `mapstructure:"check-tx-leaks" json:"check-tx-leaks" yaml:"check-tx-leaks,omitempty"`
}

// SQLBuildersConfig defines which SQL builder libraries to analyze.
type SQLBuildersConfig struct {
	// Squirrel enables checking github.com/Masterminds/squirrel
	Squirrel bool `mapstructure:"squirrel" json:"squirrel" yaml:"squirrel"`

	// GORM enables checking gorm.io/gorm
	GORM bool `mapstructure:"gorm" json:"gorm" yaml:"gorm"`

	// SQLx enables checking github.com/jmoiron/sqlx
	SQLx bool `mapstructure:"sqlx" json:"sqlx" yaml:"sqlx"`

	// Ent enables checking entgo.io/ent
	Ent bool `mapstructure:"ent" json:"ent" yaml:"ent"`

	// PGX enables checking github.com/jackc/pgx
	PGX bool `mapstructure:"pgx" json:"pgx" yaml:"pgx"`

	// Bun enables checking github.com/uptrace/bun
	Bun bool `mapstructure:"bun" json:"bun" yaml:"bun"`

	// SQLBoiler enables checking github.com/volatiletech/sqlboiler
	SQLBoiler bool `mapstructure:"sqlboiler" json:"sqlboiler" yaml:"sqlboiler"`

	// Jet enables checking github.com/go-jet/jet
	Jet bool `mapstructure:"jet" json:"jet" yaml:"jet"`

	// Sqlc enables checking github.com/sqlc-dev/sqlc generated code
	Sqlc bool `mapstructure:"sqlc" json:"sqlc" yaml:"sqlc"`

	// Goqu enables checking github.com/doug-martin/goqu
	Goqu bool `mapstructure:"goqu" json:"goqu" yaml:"goqu"`

	// Rel enables checking github.com/go-rel/rel
	Rel bool `mapstructure:"rel" json:"rel" yaml:"rel"`

	// Reform enables checking gopkg.in/reform.v1
	Reform bool `mapstructure:"reform" json:"reform" yaml:"reform"`
}

// DefaultSQLBuildersConfig returns the default SQL builders configuration with all checkers enabled.
func DefaultSQLBuildersConfig() SQLBuildersConfig {
	return SQLBuildersConfig{
		Squirrel:  true,
		GORM:      true,
		SQLx:      true,
		Ent:       true,
		PGX:       true,
		Bun:       true,
		SQLBoiler: true,
		Jet:       true,
		Sqlc:      true,
		Goqu:      true,
		Rel:       true,
		Reform:    true,
	}
}

// DefaultSettings returns the default configuration for unqueryvet.
// By default, all detection features are enabled for maximum coverage.
func DefaultSettings() UnqueryvetSettings {
	return UnqueryvetSettings{
		CheckSQLBuilders:             true,
		CheckAliasedWildcard:         true,
		CheckStringConcat:            true,
		CheckFormatStrings:           true,
		CheckStringBuilder:           true,
		CheckSubqueries:              true,
		N1DetectionEnabled:           true,
		SQLInjectionDetectionEnabled: true,
		TxLeakDetectionEnabled:       true,
		Severity:                     "warning",
		AllowedPatterns: []string{
			`(?i)COUNT\(\s*\*\s*\)`,
			`(?i)MAX\(\s*\*\s*\)`,
			`(?i)MIN\(\s*\*\s*\)`,
			`(?i)SELECT \* FROM information_schema\..*`,
			`(?i)SELECT \* FROM pg_catalog\..*`,
			`(?i)SELECT \* FROM sys\..*`,
		},
		IgnoredFunctions: []string{},
		IgnoredFiles:     []string{},
		SQLBuilders:      DefaultSQLBuildersConfig(),
		Rules: RuleSeverity{
			"select-star":   "warning",
			"n1-queries":    "warning",
			"sql-injection": "error",
			"tx-leak":       "warning",
		},
	}
}
