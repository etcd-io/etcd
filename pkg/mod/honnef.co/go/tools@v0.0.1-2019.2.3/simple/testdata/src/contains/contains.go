package pkg

import (
	"bytes"
	"strings"
)

func fn() {
	_ = strings.IndexRune("", 'x') > -1 // want ` strings\.ContainsRune`
	_ = strings.IndexRune("", 'x') >= 0 // want ` strings\.ContainsRune`
	_ = strings.IndexRune("", 'x') > 0
	_ = strings.IndexRune("", 'x') >= -1
	_ = strings.IndexRune("", 'x') != -1 // want ` strings\.ContainsRune`
	_ = strings.IndexRune("", 'x') == -1 // want `!strings\.ContainsRune`
	_ = strings.IndexRune("", 'x') != 0
	_ = strings.IndexRune("", 'x') < 0 // want `!strings\.ContainsRune`

	_ = strings.IndexAny("", "") > -1 // want ` strings\.ContainsAny`
	_ = strings.IndexAny("", "") >= 0 // want ` strings\.ContainsAny`
	_ = strings.IndexAny("", "") > 0
	_ = strings.IndexAny("", "") >= -1
	_ = strings.IndexAny("", "") != -1 // want ` strings\.ContainsAny`
	_ = strings.IndexAny("", "") == -1 // want `!strings\.ContainsAny`
	_ = strings.IndexAny("", "") != 0
	_ = strings.IndexAny("", "") < 0 // want `!strings\.ContainsAny`

	_ = strings.Index("", "") > -1 // want ` strings\.Contains`
	_ = strings.Index("", "") >= 0 // want ` strings\.Contains`
	_ = strings.Index("", "") > 0
	_ = strings.Index("", "") >= -1
	_ = strings.Index("", "") != -1 // want ` strings\.Contains`
	_ = strings.Index("", "") == -1 // want `!strings\.Contains`
	_ = strings.Index("", "") != 0
	_ = strings.Index("", "") < 0 // want `!strings\.Contains`

	_ = bytes.IndexRune(nil, 'x') > -1 // want ` bytes\.ContainsRune`
	_ = bytes.IndexAny(nil, "") > -1   // want ` bytes\.ContainsAny`
	_ = bytes.Index(nil, nil) > -1     // want ` bytes\.Contains`
}
