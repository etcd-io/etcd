package ll

import (
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	"github.com/olekukonko/ll/lx"
)

// Inspector is a utility for Logger that provides advanced inspection and logging of data
// in human-readable JSON format. It uses reflection to access and represent unexported fields,
// nested structs, embedded structs, and pointers, making it useful for debugging complex data structures.
type Inspector struct {
	logger *Logger
}

// NewInspector returns a new Inspector instance associated with the provided logger.
func NewInspector(logger *Logger) *Inspector {
	return &Inspector{logger: logger}
}

// Log outputs the given values as indented JSON at the Info level, prefixed with the caller's
// file name and line number. It handles structs (including unexported fields, nested, and embedded),
// pointers, errors, and other types. The skip parameter determines how many stack frames to skip
// when identifying the caller; typically set to 2 to account for the call to Log and its wrapper.
//
// Example usage within a Logger method:
//
//	o := NewInspector(l)
//	o.Log(2, someStruct)
func (o *Inspector) Log(skip int, values ...interface{}) {
	// Skip if logger is suspended or Info level is disabled
	if o.logger.suspend.Load() || !o.logger.shouldLog(lx.LevelInfo) {
		return
	}

	// Retrieve caller information for logging context
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		o.logger.log(lx.LevelError, lx.ClassText, "Inspector: Unable to parse runtime caller", nil, false)
		return
	}

	// Extract short filename for concise output
	shortFile := file
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		shortFile = file[idx+1:]
	}

	// Process each value individually
	for _, value := range values {
		var jsonData []byte
		var err error

		// Use reflection for struct types to handle unexported and nested fields
		val := reflect.ValueOf(value)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() == reflect.Struct {
			valueMap := o.structToMap(val)
			jsonData, err = json.MarshalIndent(valueMap, "", "  ")
		} else if errVal, ok := value.(error); ok {
			// Special handling for errors to represent them as a simple map
			value = map[string]string{"error": errVal.Error()}
			jsonData, err = json.MarshalIndent(value, "", "  ")
		} else {
			// Fall back to standard JSON marshaling for non-struct types
			jsonData, err = json.MarshalIndent(value, "", "  ")
		}

		if err != nil {
			o.logger.log(lx.LevelError, lx.ClassInspect, fmt.Sprintf("Inspector: JSON encoding error: %v", err), nil, false)
			continue
		}

		// Construct log message with file, line, and JSON data
		msg := fmt.Sprintf("[%s:%d] %s", shortFile, line, string(jsonData))
		o.logger.log(lx.LevelInfo, lx.ClassInspect, msg, nil, false)
	}
}

// structToMap recursively converts a struct's reflect.Value to a map[string]interface{}.
// It includes unexported fields (named with parentheses), prefixes pointers with '*',
// flattens anonymous embedded structs without json tags, and uses unsafe pointers to access
// unexported primitive fields when reflect.CanInterface() returns false.
func (o *Inspector) structToMap(val reflect.Value) map[string]interface{} {
	result := make(map[string]interface{})
	if !val.IsValid() {
		return result
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Determine field name: prefer json tag if present and not "-", else use struct field name
		baseName := fieldType.Name
		jsonTag := fieldType.Tag.Get("json")
		hasJsonTag := false
		if jsonTag != "" {
			if idx := strings.Index(jsonTag, ","); idx != -1 {
				jsonTag = jsonTag[:idx]
			}
			if jsonTag != "-" {
				baseName = jsonTag
				hasJsonTag = true
			}
		}

		// Enclose unexported field names in parentheses
		fieldName := baseName
		if !fieldType.IsExported() {
			fieldName = "(" + baseName + ")"
		}

		// Handle pointer fields
		isPtr := fieldType.Type.Kind() == reflect.Ptr
		if isPtr {
			fieldName = "*" + fieldName
			if field.IsNil() {
				result[fieldName] = nil
				continue
			}
			field = field.Elem()
		}

		// Recurse for struct fields
		if field.Kind() == reflect.Struct {
			subMap := o.structToMap(field)
			isNested := !fieldType.Anonymous || hasJsonTag
			if isNested {
				result[fieldName] = subMap
			} else {
				// Flatten embedded struct fields into the parent map, avoiding overwrites
				for k, v := range subMap {
					if _, exists := result[k]; !exists {
						result[k] = v
					}
				}
			}
		} else {
			// Handle primitive fields
			if field.CanInterface() {
				result[fieldName] = field.Interface()
			} else {
				// Use unsafe access for unexported primitives
				ptr := getDataPtr(field)
				switch field.Kind() {
				case reflect.String:
					result[fieldName] = *(*string)(ptr)
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					result[fieldName] = o.getIntFromUnexportedField(field)
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					result[fieldName] = o.getUintFromUnexportedField(field)
				case reflect.Float32, reflect.Float64:
					result[fieldName] = o.getFloatFromUnexportedField(field)
				case reflect.Bool:
					result[fieldName] = *(*bool)(ptr)
				default:
					result[fieldName] = fmt.Sprintf("*unexported %s*", field.Type().String())
				}
			}
		}
	}
	return result
}

// emptyInterface represents the internal structure of an empty interface{}.
// This is used for unsafe pointer manipulation to access unexported field data.
type emptyInterface struct {
	typ  unsafe.Pointer
	word unsafe.Pointer
}

// getDataPtr returns an unsafe.Pointer to the underlying data of a reflect.Value.
// This enables direct access to unexported fields via unsafe operations.
func getDataPtr(v reflect.Value) unsafe.Pointer {
	return (*emptyInterface)(unsafe.Pointer(&v)).word
}

// getIntFromUnexportedField extracts a signed integer value from an unexported field
// using unsafe pointer access. It supports int, int8, int16, int32, and int64 kinds,
// returning the value as int64. Returns 0 for unsupported kinds.
func (o *Inspector) getIntFromUnexportedField(field reflect.Value) int64 {
	ptr := getDataPtr(field)
	switch field.Kind() {
	case reflect.Int:
		return int64(*(*int)(ptr))
	case reflect.Int8:
		return int64(*(*int8)(ptr))
	case reflect.Int16:
		return int64(*(*int16)(ptr))
	case reflect.Int32:
		return int64(*(*int32)(ptr))
	case reflect.Int64:
		return *(*int64)(ptr)
	}
	return 0
}

// getUintFromUnexportedField extracts an unsigned integer value from an unexported field
// using unsafe pointer access. It supports uint, uint8, uint16, uint32, and uint64 kinds,
// returning the value as uint64. Returns 0 for unsupported kinds.
func (o *Inspector) getUintFromUnexportedField(field reflect.Value) uint64 {
	ptr := getDataPtr(field)
	switch field.Kind() {
	case reflect.Uint:
		return uint64(*(*uint)(ptr))
	case reflect.Uint8:
		return uint64(*(*uint8)(ptr))
	case reflect.Uint16:
		return uint64(*(*uint16)(ptr))
	case reflect.Uint32:
		return uint64(*(*uint32)(ptr))
	case reflect.Uint64:
		return *(*uint64)(ptr)
	}
	return 0
}

// getFloatFromUnexportedField extracts a floating-point value from an unexported field
// using unsafe pointer access. It supports float32 and float64 kinds, returning the value
// as float64. Returns 0 for unsupported kinds.
func (o *Inspector) getFloatFromUnexportedField(field reflect.Value) float64 {
	ptr := getDataPtr(field)
	switch field.Kind() {
	case reflect.Float32:
		return float64(*(*float32)(ptr))
	case reflect.Float64:
		return *(*float64)(ptr)
	}
	return 0
}
