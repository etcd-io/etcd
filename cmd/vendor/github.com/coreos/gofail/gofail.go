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

// gofail is a tool for enabling/disabling failpoints in go code.
package main

import (
	"fmt"
	"go/build"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coreos/gofail/code"
)

type xfrmFunc func(io.Writer, io.Reader) ([]*code.Failpoint, error)

func xfrmFile(xfrm xfrmFunc, path string) ([]*code.Failpoint, error) {
	src, serr := os.Open(path)
	if serr != nil {
		return nil, serr
	}
	defer src.Close()

	dst, derr := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if derr != nil {
		return nil, derr
	}
	defer dst.Close()

	fps, xerr := xfrm(dst, src)
	if xerr != nil || len(fps) == 0 {
		os.Remove(dst.Name())
		return nil, xerr
	}

	return fps, os.Rename(dst.Name(), path)
}

func dir2files(dir, ext string) (ret []string, err error) {
	if dir, err = filepath.Abs(dir); err != nil {
		return nil, err
	}

	f, ferr := os.Open(dir)
	if ferr != nil {
		return nil, ferr
	}
	defer f.Close()

	names, rerr := f.Readdirnames(0)
	if rerr != nil {
		return nil, rerr
	}
	for _, f := range names {
		if path.Ext(f) != ext {
			continue
		}
		ret = append(ret, path.Join(dir, f))
	}
	return ret, nil
}

func paths2files(paths []string) (files []string) {
	// no paths => use cwd
	if len(paths) == 0 {
		wd, gerr := os.Getwd()
		if gerr != nil {
			fmt.Println(gerr)
			os.Exit(1)
		}
		return paths2files([]string{wd})
	}
	for _, p := range paths {
		s, serr := os.Stat(p)
		if serr != nil {
			fmt.Println(serr)
			os.Exit(1)
		}
		if s.IsDir() {
			fs, err := dir2files(p, ".go")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			files = append(files, fs...)
		} else if path.Ext(s.Name()) == ".go" {
			abs, err := filepath.Abs(p)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			files = append(files, abs)
		}
	}
	return files
}

func writeBinding(file string, fps []*code.Failpoint) {
	if len(fps) == 0 {
		return
	}
	fname := strings.Split(path.Base(file), ".go")[0] + ".fail.go"
	out, err := os.Create(path.Join(path.Dir(file), fname))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// XXX: support "package main"
	pkgAbsDir := path.Dir(file)
	pkg := path.Base(pkgAbsDir)
	pkgDir := ""
	for _, srcdir := range build.Default.SrcDirs() {
		if strings.HasPrefix(pkgAbsDir, srcdir) {
			pkgDir = strings.Replace(pkgAbsDir, srcdir, "", 1)
			break
		}
	}
	fppath := pkg
	if pkgDir == "" {
		fmt.Fprintf(
			os.Stderr,
			"missing package for %q; using %q as failpoint path\n",
			pkgAbsDir,
			pkg)
	} else {
		fppath = pkgDir[1:]
	}
	code.NewBinding(pkg, fppath, fps).Write(out)
	out.Close()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("not enough arguments")
		os.Exit(1)
	}

	var xfrm xfrmFunc
	enable := false
	switch os.Args[1] {
	case "enable":
		xfrm = code.ToFailpoints
		enable = true
	case "disable":
		xfrm = code.ToComments
	default:
		fmt.Println("expected enable or disable")
	}

	files := paths2files(os.Args[2:])
	fps := [][]*code.Failpoint{}
	for _, path := range files {
		curfps, err := xfrmFile(xfrm, path)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fps = append(fps, curfps)
	}

	if enable {
		// build runtime bindings <FILE>.fail.go
		for i := range files {
			writeBinding(files[i], fps[i])
		}
	} else {
		// remove all runtime bindings
		for i := range files {
			fname := strings.Split(path.Base(files[i]), ".go")[0] + ".fail.go"
			os.Remove(path.Join(path.Dir(files[i]), fname))
		}
	}
}
