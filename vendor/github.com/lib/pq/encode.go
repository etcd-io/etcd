package pq

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq/internal/pqtime"
	"github.com/lib/pq/oid"
)

func binaryEncode(x any) ([]byte, error) {
	switch v := x.(type) {
	case []byte:
		return v, nil
	default:
		return encode(x, oid.T_unknown)
	}
}

func encode(x any, pgtypOid oid.Oid) ([]byte, error) {
	switch v := x.(type) {
	case int64:
		return strconv.AppendInt(nil, v, 10), nil
	case float64:
		return strconv.AppendFloat(nil, v, 'f', -1, 64), nil
	case []byte:
		if v == nil {
			return nil, nil
		}
		if pgtypOid == oid.T_bytea {
			return encodeBytea(v), nil
		}
		return v, nil
	case string:
		if pgtypOid == oid.T_bytea {
			return encodeBytea([]byte(v)), nil
		}
		return []byte(v), nil
	case bool:
		return strconv.AppendBool(nil, v), nil
	case time.Time:
		return formatTS(v), nil
	default:
		return nil, fmt.Errorf("pq: encode: unknown type for %T", v)
	}
}

func decode(ps *parameterStatus, s []byte, typ oid.Oid, f format) (any, error) {
	switch f {
	case formatBinary:
		return binaryDecode(s, typ)
	case formatText:
		return textDecode(ps, s, typ)
	default:
		panic("unreachable")
	}
}

func binaryDecode(s []byte, typ oid.Oid) (any, error) {
	switch typ {
	case oid.T_bytea:
		return s, nil
	case oid.T_int8:
		return int64(binary.BigEndian.Uint64(s)), nil
	case oid.T_int4:
		return int64(int32(binary.BigEndian.Uint32(s))), nil
	case oid.T_int2:
		return int64(int16(binary.BigEndian.Uint16(s))), nil
	case oid.T_uuid:
		return decodeUUIDBinary(s)
	default:
		return nil, fmt.Errorf("pq: don't know how to decode binary parameter of type %d", uint32(typ))
	}

}

// decodeUUIDBinary interprets the binary format of a uuid, returning it in text format.
func decodeUUIDBinary(src []byte) ([]byte, error) {
	if len(src) != 16 {
		return nil, fmt.Errorf("pq: unable to decode uuid; bad length: %d", len(src))
	}

	dst := make([]byte, 36)
	dst[8], dst[13], dst[18], dst[23] = '-', '-', '-', '-'
	hex.Encode(dst[0:], src[0:4])
	hex.Encode(dst[9:], src[4:6])
	hex.Encode(dst[14:], src[6:8])
	hex.Encode(dst[19:], src[8:10])
	hex.Encode(dst[24:], src[10:16])
	return dst, nil
}

func textDecode(ps *parameterStatus, s []byte, typ oid.Oid) (any, error) {
	switch typ {
	case oid.T_char, oid.T_bpchar, oid.T_varchar, oid.T_text:
		return string(s), nil
	case oid.T_bytea:
		b, err := parseBytea(s)
		if err != nil {
			err = errors.New("pq: " + err.Error())
		}
		return b, err
	case oid.T_timestamptz:
		return parseTS(ps.currentLocation, string(s))
	case oid.T_timestamp, oid.T_date:
		return parseTS(nil, string(s))
	case oid.T_time:
		return parseTime(typ, s)
	case oid.T_timetz:
		return parseTime(typ, s)
	case oid.T_bool:
		return s[0] == 't', nil
	case oid.T_int8, oid.T_int4, oid.T_int2:
		i, err := strconv.ParseInt(string(s), 10, 64)
		if err != nil {
			err = errors.New("pq: " + err.Error())
		}
		return i, err
	case oid.T_float4, oid.T_float8:
		// We always use 64 bit parsing, regardless of whether the input text is for
		// a float4 or float8, because clients expect float64s for all float datatypes
		// and returning a 32-bit parsed float64 produces lossy results.
		f, err := strconv.ParseFloat(string(s), 64)
		if err != nil {
			err = errors.New("pq: " + err.Error())
		}
		return f, err
	}
	return s, nil
}

// appendEncodedText encodes item in text format as required by COPY
// and appends to buf
func appendEncodedText(buf []byte, x any) ([]byte, error) {
	switch v := x.(type) {
	case int64:
		return strconv.AppendInt(buf, v, 10), nil
	case float64:
		return strconv.AppendFloat(buf, v, 'f', -1, 64), nil
	case []byte:
		encodedBytea := encodeBytea(v)
		return appendEscapedText(buf, string(encodedBytea)), nil
	case string:
		return appendEscapedText(buf, v), nil
	case bool:
		return strconv.AppendBool(buf, v), nil
	case time.Time:
		return append(buf, formatTS(v)...), nil
	case nil:
		return append(buf, `\N`...), nil
	default:
		return nil, fmt.Errorf("pq: encode: unknown type for %T", v)
	}
}

func appendEscapedText(buf []byte, text string) []byte {
	escapeNeeded := false
	startPos := 0

	// check if we need to escape
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\\' || c == '\n' || c == '\r' || c == '\t' {
			escapeNeeded = true
			startPos = i
			break
		}
	}
	if !escapeNeeded {
		return append(buf, text...)
	}

	// copy till first char to escape, iterate the rest
	result := append(buf, text[:startPos]...)
	for i := startPos; i < len(text); i++ {
		switch c := text[i]; c {
		case '\\':
			result = append(result, '\\', '\\')
		case '\n':
			result = append(result, '\\', 'n')
		case '\r':
			result = append(result, '\\', 'r')
		case '\t':
			result = append(result, '\\', 't')
		default:
			result = append(result, c)
		}
	}
	return result
}

func parseTime(typ oid.Oid, s []byte) (time.Time, error) {
	str := string(s)

	f := "15:04:05"
	if typ == oid.T_timetz {
		f = "15:04:05-07"
		// PostgreSQL just sends the hour if the minute and second is 0:
		//   22:04:59+00
		//   22:04:59+08
		//   22:04:59+08:30
		//   22:04:59+08:30:40
		//   23:00:00.112321+02:12:13
		// So add those to the format string.
		c := strings.Count(str, ":")
		if c > 3 {
			f = "15:04:05-07:00:00"
		} else if c > 2 {
			f = "15:04:05-07:00"
		}
	}

	// Go doesn't parse 24:00, so manually set that to midnight on Jan 2. 24:00
	// is never with subseconds but may have a timezone:
	//   24:00:00
	//   24:00:00+08
	//   24:00:00-08:01:01
	var is2400Time bool
	if strings.HasPrefix(str, "24:00:00") {
		is2400Time = true
		if len(str) > 8 {
			str = "00:00:00" + str[8:]
		} else {
			str = "00:00:00"
		}
	}

	t, err := time.Parse(f, str)
	if err != nil {
		return time.Time{}, errors.New("pq: " + err.Error())
	}
	if is2400Time {
		t = t.Add(24 * time.Hour)
	}
	// TODO(v2): it uses UTC, which it shouldn't. But I'm afraid changing it now
	// will break people's code.
	//if typ == oid.T_time {
	//	// Don't use UTC but time.FixedZone("", 0)
	//	t = t.In(globalLocationCache.getLocation(0))
	//}
	return t, nil
}

var (
	infinityTSEnabled  = false
	infinityTSNegative time.Time
	infinityTSPositive time.Time
)

// EnableInfinityTs controls the handling of Postgres' "-infinity" and
// "infinity" "timestamp"s.
//
// If EnableInfinityTs is not called, "-infinity" and "infinity" will return
// []byte("-infinity") and []byte("infinity") respectively, and potentially
// cause error "sql: Scan error on column index 0: unsupported driver -> Scan
// pair: []uint8 -> *time.Time", when scanning into a time.Time value.
//
// Once EnableInfinityTs has been called, all connections created using this
// driver will decode Postgres' "-infinity" and "infinity" for "timestamp",
// "timestamp with time zone" and "date" types to the predefined minimum and
// maximum times, respectively.  When encoding time.Time values, any time which
// equals or precedes the predefined minimum time will be encoded to
// "-infinity".  Any values at or past the maximum time will similarly be
// encoded to "infinity".
//
// If EnableInfinityTs is called with negative >= positive, it will panic.
// Calling EnableInfinityTs after a connection has been established results in
// undefined behavior.  If EnableInfinityTs is called more than once, it will
// panic.
func EnableInfinityTs(negative time.Time, positive time.Time) {
	if infinityTSEnabled {
		panic("pq: infinity timestamp already enabled")
	}
	if !negative.Before(positive) {
		panic("pq: infinity timestamp: negative value must be smaller (before) than positive")
	}
	infinityTSEnabled = true
	infinityTSNegative = negative
	infinityTSPositive = positive
}

// Testing might want to toggle infinityTSEnabled
func disableInfinityTS() {
	infinityTSEnabled = false
}

// This is a time function specific to the Postgres default DateStyle setting
// ("ISO, MDY"), the only one we currently support. This accounts for the
// discrepancies between the parsing available with time.Parse and the Postgres
// date formatting quirks.
func parseTS(currentLocation *time.Location, str string) (any, error) {
	switch str {
	case "-infinity":
		if infinityTSEnabled {
			return infinityTSNegative, nil
		}
		return []byte(str), nil
	case "infinity":
		if infinityTSEnabled {
			return infinityTSPositive, nil
		}
		return []byte(str), nil
	}
	t, err := ParseTimestamp(currentLocation, str)
	if err != nil {
		err = errors.New("pq: " + err.Error())
	}
	return t, err
}

// ParseTimestamp parses Postgres' text format. It returns a time.Time in
// currentLocation iff that time's offset agrees with the offset sent from the
// Postgres server. Otherwise, ParseTimestamp returns a time.Time with the fixed
// offset offset provided by the Postgres server.
func ParseTimestamp(currentLocation *time.Location, str string) (time.Time, error) {
	return pqtime.Parse(currentLocation, str)
}

// formatTS formats t into a format postgres understands.
func formatTS(t time.Time) []byte {
	if infinityTSEnabled {
		// t <= -infinity : ! (t > -infinity)
		if !t.After(infinityTSNegative) {
			return []byte("-infinity")
		}
		// t >= infinity : ! (!t < infinity)
		if !t.Before(infinityTSPositive) {
			return []byte("infinity")
		}
	}
	return FormatTimestamp(t)
}

// FormatTimestamp formats t into Postgres' text format for timestamps.
func FormatTimestamp(t time.Time) []byte {
	return pqtime.Format(t)
}

// Parse a bytea value received from the server.  Both "hex" and the legacy
// "escape" format are supported.
func parseBytea(s []byte) (result []byte, err error) {
	// Hex format.
	if len(s) >= 2 && bytes.Equal(s[:2], []byte("\\x")) {
		s = s[2:] // trim off leading "\\x"
		result = make([]byte, hex.DecodedLen(len(s)))
		_, err := hex.Decode(result, s)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// Escape format.
	for len(s) > 0 {
		if s[0] == '\\' {
			// escaped '\\'
			if len(s) >= 2 && s[1] == '\\' {
				result = append(result, '\\')
				s = s[2:]
				continue
			}

			// '\\' followed by an octal number
			if len(s) < 4 {
				return nil, fmt.Errorf("invalid bytea sequence %v", s)
			}
			r, err := strconv.ParseUint(string(s[1:4]), 8, 8)
			if err != nil {
				return nil, fmt.Errorf("could not parse bytea value: %w", err)
			}
			result = append(result, byte(r))
			s = s[4:]
		} else {
			// We hit an unescaped, raw byte.  Try to read in as many as
			// possible in one go.
			i := bytes.IndexByte(s, '\\')
			if i == -1 {
				result = append(result, s...)
				break
			}
			result = append(result, s[:i]...)
			s = s[i:]
		}
	}
	return result, nil
}

func encodeBytea(v []byte) (result []byte) {
	result = make([]byte, 2+hex.EncodedLen(len(v)))
	result[0] = '\\'
	result[1] = 'x'
	hex.Encode(result[2:], v)
	return result
}
