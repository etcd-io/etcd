// +build !windows,!js

package runewidth

import (
	"os"
	"regexp"
	"strings"
)

var reLoc = regexp.MustCompile(`^[a-z][a-z][a-z]?(?:_[A-Z][A-Z])?\.(.+)`)

func IsEastAsian() bool {
	locale := os.Getenv("LC_CTYPE")
	if locale == "" {
		locale = os.Getenv("LANG")
	}

	// ignore C locale
	if locale == "POSIX" || locale == "C" {
		return false
	}
	if len(locale) > 1 && locale[0] == 'C' && (locale[1] == '.' || locale[1] == '-') {
		return false
	}

	charset := strings.ToLower(locale)
	r := reLoc.FindStringSubmatch(locale)
	if len(r) == 2 {
		charset = strings.ToLower(r[1])
	}

	if strings.HasSuffix(charset, "@cjk_narrow") {
		return false
	}

	for pos, b := range []byte(charset) {
		if b == '@' {
			charset = charset[:pos]
			break
		}
	}

	mbc_max := 1
	switch charset {
	case "utf-8", "utf8":
		mbc_max = 6
	case "jis":
		mbc_max = 8
	case "eucjp":
		mbc_max = 3
	case "euckr", "euccn":
		mbc_max = 2
	case "sjis", "cp932", "cp51932", "cp936", "cp949", "cp950":
		mbc_max = 2
	case "big5":
		mbc_max = 2
	case "gbk", "gb2312":
		mbc_max = 2
	}

	if mbc_max > 1 && (charset[0] != 'u' ||
		strings.HasPrefix(locale, "ja") ||
		strings.HasPrefix(locale, "ko") ||
		strings.HasPrefix(locale, "zh")) {
		return true
	}
	return false
}
