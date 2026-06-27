// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package code

import (
	"fmt"
	"io"
	"strings"
)

type Failpoint struct {
	name    string
	varType string
	code    []string

	// whitespace for padding
	ws string
}

// newFailpoint makes a new failpoint based on the a line containing a
// failpoint comment header.
func newFailpoint(l string) (*Failpoint, error) {
	lt := strings.TrimSpace(l)
	isGoFail := strings.HasPrefix(lt, pfxGofail)
	if !isGoFail {
		// not a failpoint
		return nil, nil
	}
	pfx := pfxGofail
	cmd := strings.SplitAfter(l, pfx)[1]
	fields := strings.Fields(cmd)
	if len(fields) != 3 || fields[0] != "var" {
		return nil, fmt.Errorf("failpoint: malformed comment header %q", l)
	}
	return &Failpoint{name: fields[1], varType: fields[2], ws: strings.Split(l, "//")[0]}, nil
}

// flush writes the failpoint code to a buffer
func (fp *Failpoint) flush(dst io.Writer) error {
	if len(fp.code) == 0 {
		return fp.flushSingle(dst)
	}
	return fp.flushMulti(dst)
}

func (fp *Failpoint) hdr(varname string) string {
	ev := errVarGoFail

	hdr := fp.ws + "if v" + fp.name + fmt.Sprintf(", %s := ", ev) + fp.Runtime() + ".Acquire();" + fmt.Sprintf(" %s == nil { ", ev)

	if fp.varType == "struct{}" {
		// unused
		varname = "_"
	}
	return hdr + varname + ", __fpTypeOK := v" + fp.name +
		".(" + fp.varType + "); if !__fpTypeOK { goto __badType" + fp.name + "} "
}

func (fp *Failpoint) footer() string {
	return "; goto __nomock" + fp.name + "; __badType" + fp.name + ": " +
		fp.Runtime() + ".BadType(v" + fp.name + ", \"" + fp.varType + "\"); __nomock" + fp.name + ": };"
}

func (fp *Failpoint) flushSingle(dst io.Writer) error {
	if _, err := io.WriteString(dst, fp.hdr("_")); err != nil {
		return err
	}
	_, err := io.WriteString(dst, fp.footer()+"\n")
	return err
}

func (fp *Failpoint) flushMulti(dst io.Writer) error {
	hdr := fp.hdr(fp.name) + "\n"
	if _, err := io.WriteString(dst, hdr); err != nil {
		return err
	}
	for _, code := range fp.code[:len(fp.code)-1] {
		if _, err := io.WriteString(dst, code+"\n"); err != nil {
			return err
		}
	}
	code := fp.code[len(fp.code)-1]
	_, err := io.WriteString(dst, code+fp.footer()+"\n")
	return err
}

func (fp *Failpoint) Name() string    { return fp.name }
func (fp *Failpoint) Runtime() string { return "__fp_" + fp.name }
