package lx

// Formatting constants for log output.
// These constants define the characters used to format log messages, ensuring consistency
// across handlers (e.g., text, JSON, colorized). They are used to construct namespace paths,
// level indicators, and field separators in log entries.
const (
	Space        = " "  // Single space for separating elements (e.g., between level and message)
	DoubleSpace  = "  " // Double space for indentation (e.g., for hierarchical output)
	Slash        = "/"  // Separator for namespace paths (e.g., "parent/child")
	Arrow        = "→"  // Arrow for NestedPath style namespaces (e.g., [parent]→[child])
	LeftBracket  = "["  // Opening bracket for namespaces and fields (e.g., [app])
	RightBracket = "]"  // Closing bracket for namespaces and fields (e.g., [app])
	Colon        = ":"  // Separator after namespace or level (e.g., [app]: INFO:) can also be "|"
	Dot          = "."  // Separator for namespace paths (e.g., "parent.child")
	Newline      = "\n" // Newline for separating log entries or stack trace lines
)

// DefaultEnabled defines the default logging state (disabled).
// It specifies whether logging is enabled by default for new Logger instances in the ll package.
// Set to false to prevent logging until explicitly enabled.
const (
	DefaultEnabled = true // Default state for new loggers (disabled)
)

// Log level constants, ordered by increasing severity.
// These constants define the severity levels for log messages, used to filter logs based
// on the logger’s minimum level. They are ordered to allow comparison (e.g., LevelDebug < LevelWarn).
const (
	LevelNone    LevelType = iota // Debug level for detailed diagnostic information
	LevelInfo                     // Info level for general operational messages
	LevelWarn                     // Warn level for warning conditions
	LevelError                    // Error level for error conditions requiring attention
	LevelFatal                    // Fatal level for critical error conditions
	LevelDebug                    // None level for logs without a specific severity (e.g., raw output)
	LevelUnknown                  // None level for logs without a specific severity (e.g., raw output)
)

// String constants for each level
const (
	DebugString   = "DEBUG"
	InfoString    = "INFO"
	WarnString    = "WARN"
	WarningString = "WARNING"
	ErrorString   = "ERROR"
	FatalString   = "FATAL"
	NoneString    = "NONE"
	UnknownString = "UNKNOWN"

	TextString    = "TEXT"
	JSONString    = "JSON"
	DumpString    = "DUMP"
	SpecialString = "SPECIAL"
	RawString     = "RAW"
	InspectString = "INSPECT"
	DbgString     = "DBG"
	TimedString   = "TIMED"
)

// Log class constants, defining the type of log entry.
// These constants categorize log entries by their content or purpose, influencing how
// handlers process them (e.g., text, JSON, hex dump).
const (
	ClassText    ClassType = iota // Text entries for standard log messages
	ClassJSON                     // JSON entries for structured output
	ClassDump                     // Dump entries for hex/ASCII dumps
	ClassSpecial                  // Special entries for custom or non-standard logs
	ClassRaw                      // Raw entries for unformatted output
	ClassInspect                  // Inspect entries for debugging
	ClassDbg                      // Inspect entries for debugging
	ClassTimed                    // Inspect entries for debugging
	ClassUnknown                  // Unknown output
)

// Namespace style constants.
// These constants define how namespace paths are formatted in log output, affecting the
// visual representation of hierarchical namespaces.
const (
	FlatPath   StyleType = iota // Formats namespaces as [parent/child]
	NestedPath                  // Formats namespaces as [parent]→[child]
)
