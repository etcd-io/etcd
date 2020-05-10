// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/token"
	"strconv"
	"strings"
)

func parseOctothorpDecimal(s string) int {
	if s != "" && s[0] == '#' {
		if s, err := strconv.ParseInt(s[1:], 10, 32); err == nil {
			return int(s)
		}
	}
	return -1
}

func parsePos(pos string) (filename string, startOffset, endOffset int, err error) {
	if pos == "" {
		err = fmt.Errorf("no source position specified")
		return
	}

	colon := strings.LastIndex(pos, ":")
	if colon < 0 {
		err = fmt.Errorf("bad position syntax %q", pos)
		return
	}
	filename, offset := pos[:colon], pos[colon+1:]
	startOffset = -1
	endOffset = -1
	if hyphen := strings.Index(offset, ","); hyphen < 0 {
		// e.g. "foo.go:#123"
		startOffset = parseOctothorpDecimal(offset)
		endOffset = startOffset
	} else {
		// e.g. "foo.go:#123,#456"
		startOffset = parseOctothorpDecimal(offset[:hyphen])
		endOffset = parseOctothorpDecimal(offset[hyphen+1:])
	}
	if startOffset < 0 || endOffset < 0 {
		err = fmt.Errorf("invalid offset %q in query position", offset)
		return
	}
	return
}

func fileOffsetToPos(file *token.File, startOffset, endOffset int) (start, end token.Pos, err error) {
	// Range check [start..end], inclusive of both end-points.

	if 0 <= startOffset && startOffset <= file.Size() {
		start = file.Pos(int(startOffset))
	} else {
		err = fmt.Errorf("start position is beyond end of file")
		return
	}

	if 0 <= endOffset && endOffset <= file.Size() {
		end = file.Pos(int(endOffset))
	} else {
		err = fmt.Errorf("end position is beyond end of file")
		return
	}

	return
}
