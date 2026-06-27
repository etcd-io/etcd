package lx

import (
	"strings"
	"time"
)

// LevelType represents the severity of a log message.
// It is an integer type used to define log levels (Debug, Info, Warn, Error, None), with associated
// string representations for display in log output.
type LevelType int

// String converts a LevelType to its string representation.
// It maps each level constant to a human-readable string, returning "UNKNOWN" for invalid levels.
// Used by handlers to display the log level in output.
// Example:
//
//	var level lx.LevelType = lx.LevelInfo
//	fmt.Println(level.String()) // Output: INFO
func (l LevelType) String() string {
	switch l {
	case LevelDebug:
		return DebugString
	case LevelInfo:
		return InfoString
	case LevelWarn:
		return WarnString
	case LevelError:
		return ErrorString
	case LevelFatal:
		return FatalString
	case LevelNone:
		return NoneString
	default:
		return UnknownString
	}
}

func (l LevelType) Name(class ClassType) string {
	if class == ClassRaw || class == ClassDump || class == ClassInspect || class == ClassDbg || class == ClassTimed {
		return class.String()
	}
	return l.String()
}

// LevelParse converts a string to its corresponding LevelType.
// It parses a string (case-insensitive) and returns the corresponding LevelType, defaulting to
// LevelUnknown for unrecognized strings. Supports "WARNING" as an alias for "WARN".
func LevelParse(s string) LevelType {
	switch strings.ToUpper(s) {
	case DebugString:
		return LevelDebug
	case InfoString:
		return LevelInfo
	case WarnString, WarningString: // Allow both "WARN" and "WARNING"
		return LevelWarn
	case ErrorString:
		return LevelError
	case NoneString:
		return LevelNone
	default:
		return LevelUnknown
	}
}

// Entry represents a single log entry passed to handlers.
// It encapsulates all information about a log message, including its timestamp, severity,
// content, namespace, metadata, and formatting style. Handlers process Entry instances
// to produce formatted output (e.g., text, JSON). The struct is immutable once created,
// ensuring thread-safety in handler processing.
type Entry struct {
	Timestamp time.Time // Time the log was created
	Level     LevelType // Severity level of the log (Debug, Info, Warn, Error, None)
	Message   string    // Log message content
	Namespace string    // Namespace path (e.g., "parent/child")
	Fields    Fields    // Additional key-value metadata (e.g., {"user": "alice"})
	Style     StyleType // Namespace formatting style (FlatPath or NestedPath)
	Error     error     // Associated error, if any (e.g., for error logs)
	Class     ClassType // Type of log entry (Text, JSON, Dump, Special, Raw)
	Stack     []byte    // Stack trace data (if present)
	Id        int       `json:"-"` // Unique ID for the entry, ignored in JSON output
}

// StyleType defines how namespace paths are formatted in log output.
// It is an integer type used to select between FlatPath ([parent/child]) and NestedPath
// ([parent]→[child]) styles, affecting how handlers render namespace hierarchies.
type StyleType int

// ClassType represents the type of a log entry.
// It is an integer type used to categorize log entries (Text, JSON, Dump, Special, Raw),
// influencing how handlers process and format them.
type ClassType int

// String converts a ClassType to its string representation.
// It maps each class constant to a human-readable string, returning "UNKNOWN" for invalid classes.
// Used by handlers to indicate the entry type in output (e.g., JSON fields).
// Example:
//
//	var class lx.ClassType = lx.ClassText
//	fmt.Println(class.String()) // Output: TEST
func (t ClassType) String() string {
	switch t {
	case ClassText:

		return TextString
	case ClassJSON:

		return JSONString
	case ClassDump:
		return DumpString
	case ClassSpecial:
		return SpecialString
	case ClassInspect:
		return InspectString
	case ClassDbg:
		return DbgString
	case ClassRaw:
		return RawString
	case ClassTimed:
		return TimedString
	default:
		return UnknownString
	}
}

// ParseClass converts a string to its corresponding ClassType.
// It parses a string (case-insensitive) and returns the corresponding ClassType, defaulting to
// ClassUnknown for unrecognized strings.
func ParseClass(s string) ClassType {
	switch strings.ToUpper(s) {
	case TextString:
		return ClassText
	case JSONString:
		return ClassJSON
	case DumpString:
		return ClassDump
	case SpecialString:
		return ClassSpecial
	case RawString:
		return ClassRaw
	default:
		return ClassUnknown
	}
}
