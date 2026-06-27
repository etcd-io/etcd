package jsonschema

import (
	"net/netip"
	gourl "net/url"
	"strconv"
	"strings"
	"time"
)

// Format defined specific format.
type Format struct {
	// Name of format.
	Name string

	// Validate checks if given value is of this format.
	Validate func(v any) error
}

var formats = map[string]*Format{
	"json-pointer":          {"json-pointer", validateJSONPointer},
	"relative-json-pointer": {"relative-json-pointer", validateRelativeJSONPointer},
	"uuid":                  {"uuid", validateUUID},
	"duration":              {"duration", validateDuration},
	"period":                {"period", validatePeriod},
	"ipv4":                  {"ipv4", validateIPV4},
	"ipv6":                  {"ipv6", validateIPV6},
	"hostname":              {"hostname", validateHostname},
	"email":                 {"email", validateEmail},
	"date":                  {"date", validateDate},
	"time":                  {"time", validateTime},
	"date-time":             {"date-time", validateDateTime},
	"uri":                   {"uri", validateURI},
	"iri":                   {"iri", validateURI},
	"uri-reference":         {"uri-reference", validateURIReference},
	"iri-reference":         {"iri-reference", validateURIReference},
	"uri-template":          {"uri-template", validateURITemplate},
	"semver":                {"semver", validateSemver},
}

// see https://www.rfc-editor.org/rfc/rfc6901#section-3
func validateJSONPointer(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if s == "" {
		return nil
	}
	if !strings.HasPrefix(s, "/") {
		return LocalizableError("not starting with /")
	}
	for _, tok := range strings.Split(s, "/")[1:] {
		escape := false
		for _, ch := range tok {
			if escape {
				escape = false
				if ch != '0' && ch != '1' {
					return LocalizableError("~ must be followed by 0 or 1")
				}
				continue
			}
			if ch == '~' {
				escape = true
				continue
			}
			switch {
			case ch >= '\x00' && ch <= '\x2E':
			case ch >= '\x30' && ch <= '\x7D':
			case ch >= '\x7F' && ch <= '\U0010FFFF':
			default:
				return LocalizableError("invalid character %q", ch)
			}
		}
		if escape {
			return LocalizableError("~ must be followed by 0 or 1")
		}
	}
	return nil
}

// see https://tools.ietf.org/html/draft-handrews-relative-json-pointer-01#section-3
func validateRelativeJSONPointer(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}

	// start with non-negative-integer
	numDigits := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			numDigits++
		} else {
			break
		}
	}
	if numDigits == 0 {
		return LocalizableError("must start with non-negative integer")
	}
	if numDigits > 1 && strings.HasPrefix(s, "0") {
		return LocalizableError("starts with zero")
	}
	s = s[numDigits:]

	// followed by either json-pointer or '#'
	if s == "#" {
		return nil
	}
	return validateJSONPointer(s)
}

// see https://datatracker.ietf.org/doc/html/rfc4122#page-4
func validateUUID(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}

	hexGroups := []int{8, 4, 4, 4, 12}
	groups := strings.Split(s, "-")
	if len(groups) != len(hexGroups) {
		return LocalizableError("must have %d elements", len(hexGroups))
	}
	for i, group := range groups {
		if len(group) != hexGroups[i] {
			return LocalizableError("element %d must be %d characters long", i+1, hexGroups[i])
		}
		for _, ch := range group {
			switch {
			case ch >= '0' && ch <= '9':
			case ch >= 'a' && ch <= 'f':
			case ch >= 'A' && ch <= 'F':
			default:
				return LocalizableError("non-hex character %q", ch)
			}
		}
	}
	return nil
}

// see https://datatracker.ietf.org/doc/html/rfc3339#appendix-A
func validateDuration(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}

	// must start with 'P'
	s, ok = strings.CutPrefix(s, "P")
	if !ok {
		return LocalizableError("must start with P")
	}
	if s == "" {
		return LocalizableError("nothing after P")
	}

	// dur-week
	if s, ok := strings.CutSuffix(s, "W"); ok {
		if s == "" {
			return LocalizableError("no number in week")
		}
		for _, ch := range s {
			if ch < '0' || ch > '9' {
				return LocalizableError("invalid week")
			}
		}
		return nil
	}

	allUnits := []string{"YMD", "HMS"}
	for i, s := range strings.Split(s, "T") {
		if i != 0 && s == "" {
			return LocalizableError("no time elements")
		}
		if i >= len(allUnits) {
			return LocalizableError("more than one T")
		}
		units := allUnits[i]
		for s != "" {
			digitCount := 0
			for _, ch := range s {
				if ch >= '0' && ch <= '9' {
					digitCount++
				} else {
					break
				}
			}
			if digitCount == 0 {
				return LocalizableError("missing number")
			}
			s = s[digitCount:]
			if s == "" {
				return LocalizableError("missing unit")
			}
			unit := s[0]
			j := strings.IndexByte(units, unit)
			if j == -1 {
				if strings.IndexByte(allUnits[i], unit) != -1 {
					return LocalizableError("unit %q out of order", unit)
				}
				return LocalizableError("invalid unit %q", unit)
			}
			units = units[j+1:]
			s = s[1:]
		}
	}

	return nil
}

func validateIPV4(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	groups := strings.Split(s, ".")
	if len(groups) != 4 {
		return LocalizableError("expected four decimals")
	}
	for _, group := range groups {
		if len(group) > 1 && group[0] == '0' {
			return LocalizableError("leading zeros")
		}
		n, err := strconv.Atoi(group)
		if err != nil {
			return err
		}
		if n < 0 || n > 255 {
			return LocalizableError("decimal must be between 0 and 255")
		}
	}
	return nil
}

func validateIPV6(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if !strings.Contains(s, ":") {
		return LocalizableError("missing colon")
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return err
	}
	if addr.Zone() != "" {
		return LocalizableError("zone id is not a part of ipv6 address")
	}
	return nil
}

// see https://en.wikipedia.org/wiki/Hostname#Restrictions_on_valid_host_names
func validateHostname(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}

	// entire hostname (including the delimiting dots but not a trailing dot) has a maximum of 253 ASCII characters
	s = strings.TrimSuffix(s, ".")
	if len(s) > 253 {
		return LocalizableError("more than 253 characters long")
	}

	// Hostnames are composed of series of labels concatenated with dots, as are all domain names
	for _, label := range strings.Split(s, ".") {
		// Each label must be from 1 to 63 characters long
		if len(label) < 1 || len(label) > 63 {
			return LocalizableError("label must be 1 to 63 characters long")
		}

		// labels must not start or end with a hyphen
		if strings.HasPrefix(label, "-") {
			return LocalizableError("label starts with hyphen")
		}
		if strings.HasSuffix(label, "-") {
			return LocalizableError("label ends with hyphen")
		}

		// labels may contain only the ASCII letters 'a' through 'z' (in a case-insensitive manner),
		// the digits '0' through '9', and the hyphen ('-')
		for _, ch := range label {
			switch {
			case ch >= 'a' && ch <= 'z':
			case ch >= 'A' && ch <= 'Z':
			case ch >= '0' && ch <= '9':
			case ch == '-':
			default:
				return LocalizableError("invalid character %q", ch)
			}
		}
	}
	return nil
}

// see https://en.wikipedia.org/wiki/Email_address
func validateEmail(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	// entire email address to be no more than 254 characters long
	if len(s) > 254 {
		return LocalizableError("more than 255 characters long")
	}

	// email address is generally recognized as having two parts joined with an at-sign
	at := strings.LastIndexByte(s, '@')
	if at == -1 {
		return LocalizableError("missing @")
	}
	local, domain := s[:at], s[at+1:]

	// local part may be up to 64 characters long
	if len(local) > 64 {
		return LocalizableError("local part more than 64 characters long")
	}

	if len(local) > 1 && strings.HasPrefix(local, `"`) && strings.HasPrefix(local, `"`) {
		// quoted
		local := local[1 : len(local)-1]
		if strings.IndexByte(local, '\\') != -1 || strings.IndexByte(local, '"') != -1 {
			return LocalizableError("backslash and quote are not allowed within quoted local part")
		}
	} else {
		// unquoted
		if strings.HasPrefix(local, ".") {
			return LocalizableError("starts with dot")
		}
		if strings.HasSuffix(local, ".") {
			return LocalizableError("ends with dot")
		}

		// consecutive dots not allowed
		if strings.Contains(local, "..") {
			return LocalizableError("consecutive dots")
		}

		// check allowed chars
		for _, ch := range local {
			switch {
			case ch >= 'a' && ch <= 'z':
			case ch >= 'A' && ch <= 'Z':
			case ch >= '0' && ch <= '9':
			case strings.ContainsRune(".!#$%&'*+-/=?^_`{|}~", ch):
			default:
				return LocalizableError("invalid character %q", ch)
			}
		}
	}

	// domain if enclosed in brackets, must match an IP address
	if strings.HasPrefix(domain, "[") && strings.HasSuffix(domain, "]") {
		domain = domain[1 : len(domain)-1]
		if rem, ok := strings.CutPrefix(domain, "IPv6:"); ok {
			if err := validateIPV6(rem); err != nil {
				return LocalizableError("invalid ipv6 address: %v", err)
			}
			return nil
		}
		if err := validateIPV4(domain); err != nil {
			return LocalizableError("invalid ipv4 address: %v", err)
		}
		return nil
	}

	// domain must match the requirements for a hostname
	if err := validateHostname(domain); err != nil {
		return LocalizableError("invalid domain: %v", err)
	}

	return nil
}

// see see https://datatracker.ietf.org/doc/html/rfc3339#section-5.6
func validateDate(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	_, err := time.Parse("2006-01-02", s)
	return err
}

// see https://datatracker.ietf.org/doc/html/rfc3339#section-5.6
// NOTE: golang time package does not support leap seconds.
func validateTime(v any) error {
	str, ok := v.(string)
	if !ok {
		return nil
	}

	// min: hh:mm:ssZ
	if len(str) < 9 {
		return LocalizableError("less than 9 characters long")
	}
	if str[2] != ':' || str[5] != ':' {
		return LocalizableError("missing colon in correct place")
	}

	// parse hh:mm:ss
	var hms []int
	for _, tok := range strings.SplitN(str[:8], ":", 3) {
		i, err := strconv.Atoi(tok)
		if err != nil {
			return LocalizableError("invalid hour/min/sec")
		}
		if i < 0 {
			return LocalizableError("non-positive hour/min/sec")
		}
		hms = append(hms, i)
	}
	if len(hms) != 3 {
		return LocalizableError("missing hour/min/sec")
	}
	h, m, s := hms[0], hms[1], hms[2]
	if h > 23 || m > 59 || s > 60 {
		return LocalizableError("hour/min/sec out of range")
	}
	str = str[8:]

	// parse sec-frac if present
	if rem, ok := strings.CutPrefix(str, "."); ok {
		numDigits := 0
		for _, ch := range rem {
			if ch >= '0' && ch <= '9' {
				numDigits++
			} else {
				break
			}
		}
		if numDigits == 0 {
			return LocalizableError("no digits in second fraction")
		}
		str = rem[numDigits:]
	}

	if str != "z" && str != "Z" {
		// parse time-numoffset
		if len(str) != 6 {
			return LocalizableError("offset must be 6 characters long")
		}
		var sign int
		switch str[0] {
		case '+':
			sign = -1
		case '-':
			sign = +1
		default:
			return LocalizableError("offset must begin with plus/minus")
		}
		str = str[1:]
		if str[2] != ':' {
			return LocalizableError("missing colon in offset in correct place")
		}

		var zhm []int
		for _, tok := range strings.SplitN(str, ":", 2) {
			i, err := strconv.Atoi(tok)
			if err != nil {
				return LocalizableError("invalid hour/min in offset")
			}
			if i < 0 {
				return LocalizableError("non-positive hour/min in offset")
			}
			zhm = append(zhm, i)
		}
		zh, zm := zhm[0], zhm[1]
		if zh > 23 || zm > 59 {
			return LocalizableError("hour/min in offset out of range")
		}

		// apply timezone
		hm := (h*60 + m) + sign*(zh*60+zm)
		if hm < 0 {
			hm += 24 * 60
		}
		h, m = hm/60, hm%60
	}

	// check leap second
	if s >= 60 && (h != 23 || m != 59) {
		return LocalizableError("invalid leap second")
	}

	return nil
}

// see https://datatracker.ietf.org/doc/html/rfc3339#section-5.6
func validateDateTime(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}

	// min: yyyy-mm-ddThh:mm:ssZ
	if len(s) < 20 {
		return LocalizableError("less than 20 characters long")
	}

	if s[10] != 't' && s[10] != 'T' {
		return LocalizableError("11th character must be t or T")
	}
	if err := validateDate(s[:10]); err != nil {
		return LocalizableError("invalid date element: %v", err)
	}
	if err := validateTime(s[11:]); err != nil {
		return LocalizableError("invalid time element: %v", err)
	}
	return nil
}

func parseURL(s string) (*gourl.URL, error) {
	u, err := gourl.Parse(s)
	if err != nil {
		return nil, err
	}

	// gourl does not validate ipv6 host address
	hostName := u.Hostname()
	if strings.Contains(hostName, ":") {
		if !strings.Contains(u.Host, "[") || !strings.Contains(u.Host, "]") {
			return nil, LocalizableError("ipv6 address not enclosed in brackets")
		}
		if err := validateIPV6(hostName); err != nil {
			return nil, LocalizableError("invalid ipv6 address: %v", err)
		}
	}

	return u, nil
}

func validateURI(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	u, err := parseURL(s)
	if err != nil {
		return err
	}
	if !u.IsAbs() {
		return LocalizableError("relative url")
	}
	return nil
}

func validateURIReference(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	if strings.Contains(s, `\`) {
		return LocalizableError(`contains \`)
	}
	_, err := parseURL(s)
	return err
}

func validateURITemplate(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	u, err := parseURL(s)
	if err != nil {
		return err
	}
	for _, tok := range strings.Split(u.RawPath, "/") {
		tok, err = decode(tok)
		if err != nil {
			return LocalizableError("percent decode failed: %v", err)
		}
		want := true
		for _, ch := range tok {
			var got bool
			switch ch {
			case '{':
				got = true
			case '}':
				got = false
			default:
				continue
			}
			if got != want {
				return LocalizableError("nested curly braces")
			}
			want = !want
		}
		if !want {
			return LocalizableError("no matching closing brace")
		}
	}
	return nil
}

func validatePeriod(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}

	slash := strings.IndexByte(s, '/')
	if slash == -1 {
		return LocalizableError("missing slash")
	}

	start, end := s[:slash], s[slash+1:]
	if strings.HasPrefix(start, "P") {
		if err := validateDuration(start); err != nil {
			return LocalizableError("invalid start duration: %v", err)
		}
		if err := validateDateTime(end); err != nil {
			return LocalizableError("invalid end date-time: %v", err)
		}
	} else {
		if err := validateDateTime(start); err != nil {
			return LocalizableError("invalid start date-time: %v", err)
		}
		if strings.HasPrefix(end, "P") {
			if err := validateDuration(end); err != nil {
				return LocalizableError("invalid end duration: %v", err)
			}
		} else if err := validateDateTime(end); err != nil {
			return LocalizableError("invalid end date-time: %v", err)
		}
	}

	return nil
}

// see https://semver.org/#backusnaur-form-grammar-for-valid-semver-versions
func validateSemver(v any) error {
	s, ok := v.(string)
	if !ok {
		return nil
	}

	// build --
	if i := strings.IndexByte(s, '+'); i != -1 {
		build := s[i+1:]
		if build == "" {
			return LocalizableError("build is empty")
		}
		for _, buildID := range strings.Split(build, ".") {
			if buildID == "" {
				return LocalizableError("build identifier is empty")
			}
			for _, ch := range buildID {
				switch {
				case ch >= '0' && ch <= '9':
				case (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '-':
				default:
					return LocalizableError("invalid character %q in build identifier", ch)
				}
			}
		}
		s = s[:i]
	}

	// pre-release --
	if i := strings.IndexByte(s, '-'); i != -1 {
		preRelease := s[i+1:]
		for _, preReleaseID := range strings.Split(preRelease, ".") {
			if preReleaseID == "" {
				return LocalizableError("pre-release identifier is empty")
			}
			allDigits := true
			for _, ch := range preReleaseID {
				switch {
				case ch >= '0' && ch <= '9':
				case (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '-':
					allDigits = false
				default:
					return LocalizableError("invalid character %q in pre-release identifier", ch)
				}
			}
			if allDigits && len(preReleaseID) > 1 && preReleaseID[0] == '0' {
				return LocalizableError("pre-release numeric identifier starts with zero")
			}
		}
		s = s[:i]
	}

	// versionCore --
	versions := strings.Split(s, ".")
	if len(versions) != 3 {
		return LocalizableError("versionCore must have 3 numbers separated by dot")
	}
	names := []string{"major", "minor", "patch"}
	for i, version := range versions {
		if version == "" {
			return LocalizableError("%s is empty", names[i])
		}
		if len(version) > 1 && version[0] == '0' {
			return LocalizableError("%s starts with zero", names[i])
		}
		for _, ch := range version {
			if ch < '0' || ch > '9' {
				return LocalizableError("%s contains non-digit", names[i])
			}
		}
	}

	return nil
}
