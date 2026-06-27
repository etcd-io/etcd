////////////////////////////////////////////////////////////////////////////
// Porgram: xmlfmt.go
// Purpose: Go XML Beautify from XML string using pure string manipulation
// Authors: Antonio Sun (c) 2016-2022, All rights reserved
////////////////////////////////////////////////////////////////////////////

package xmlfmt

import (
	"html"
	"regexp"
	"runtime"
	"strings"
)

var (
	reg           = regexp.MustCompile(`<([/!]?)([^>]+?)(/?)>`)
	reXMLComments = regexp.MustCompile(`(?s)(<!--)(.*?)(-->)`)
	reSpaces      = regexp.MustCompile(`(?s)>\s+<`)
	reNewlines    = regexp.MustCompile(`\r*\n`)
	// NL is the newline string used in XML output.
	NL = "\n"
)

func init() {
	// define NL for Windows
	if runtime.GOOS == "windows" {
		NL = "\r\n"
	}
}

// FormatXML will (purly) reformat the XML string in a readable way, without any rewriting/altering the structure.
// If your XML Comments have nested tags in them, or you're not 100% sure otherwise, pass `true` as the third parameter to this function. But don't turn it on blindly, as the code has become ten times more complicated because of it.
func FormatXML(xmls, prefix, indent string, nestedTagsInComments ...bool) string {
	nestedTagsInComment := false
	if len(nestedTagsInComments) > 0 {
		nestedTagsInComment = nestedTagsInComments[0]
	}
	src := reSpaces.ReplaceAllString(xmls, "><")
	if nestedTagsInComment {
		src = reXMLComments.ReplaceAllStringFunc(src, func(m string) string {
			parts := reXMLComments.FindStringSubmatch(m)
			p2 := reNewlines.ReplaceAllString(parts[2], " ")
			return parts[1] + html.EscapeString(p2) + parts[3]
		})
	}
	rf := replaceTag(prefix, indent)
	r := prefix + reg.ReplaceAllStringFunc(src, rf)
	if nestedTagsInComment {
		r = reXMLComments.ReplaceAllStringFunc(r, func(m string) string {
			parts := reXMLComments.FindStringSubmatch(m)
			return parts[1] + html.UnescapeString(parts[2]) + parts[3]
		})
	}

	return r
}

// replaceTag returns a closure function to do 's/(?<=>)\s+(?=<)//g; s(<(/?)([^>]+?)(/?)>)($indent+=$3?0:$1?-1:1;"<$1$2$3>"."\n".("  "x$indent))ge' as in Perl
// and deal with comments as well
func replaceTag(prefix, indent string) func(string) string {
	indentLevel := 0
	lastEndElem := true
	return func(m string) string {
		// head elem
		if strings.HasPrefix(m, "<?xml") {
			return NL + prefix + strings.Repeat(indent, indentLevel) + m
		}
		// empty elem
		if strings.HasSuffix(m, "/>") {
			lastEndElem = true
			return NL + prefix + strings.Repeat(indent, indentLevel) + m
		}
		// comment elem
		if strings.HasPrefix(m, "<!") {
			return NL + prefix + strings.Repeat(indent, indentLevel) + m
		}
		// end elem
		if strings.HasPrefix(m, "</") {
			indentLevel--
			if lastEndElem {
				return NL + prefix + strings.Repeat(indent, indentLevel) + m
			}
			lastEndElem = true
			return m
		} else {
			lastEndElem = false
		}
		defer func() {
			indentLevel++
		}()
		return NL + prefix + strings.Repeat(indent, indentLevel) + m
	}
}
