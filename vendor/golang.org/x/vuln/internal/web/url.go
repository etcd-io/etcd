// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code copied from
// https://github.com/golang/go/blob/2ebe77a2fda1ee9ff6fd9a3e08933ad1ebaea039/src/cmd/go/internal/web/url.go
// TODO(https://go.dev/issue/32456): if accepted, use the new API.

package web

import (
	"errors"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

var errNotAbsolute = errors.New("path is not absolute")

// URLToFilePath converts a file-scheme url to a file path.
func URLToFilePath(u *url.URL) (string, error) {
	if u.Scheme != "file" {
		return "", errors.New("non-file URL")
	}

	checkAbs := func(path string) (string, error) {
		if !filepath.IsAbs(path) {
			return "", errNotAbsolute
		}
		return path, nil
	}

	if u.Path == "" {
		if u.Host != "" || u.Opaque == "" {
			return "", errors.New("file URL missing path")
		}
		return checkAbs(filepath.FromSlash(u.Opaque))
	}

	path, err := convertFileURLPath(u.Host, u.Path)
	if err != nil {
		return path, err
	}
	return checkAbs(path)
}

// URLFromFilePath converts the given absolute path to a URL.
func URLFromFilePath(path string) (*url.URL, error) {
	if !filepath.IsAbs(path) {
		return nil, errNotAbsolute
	}

	// If path has a Windows volume name, convert the volume to a host and prefix
	// per https://blogs.msdn.microsoft.com/ie/2006/12/06/file-uris-in-windows/.
	if vol := filepath.VolumeName(path); vol != "" {
		if strings.HasPrefix(vol, `\\`) {
			path = filepath.ToSlash(path[2:])
			i := strings.IndexByte(path, '/')

			if i < 0 {
				// A degenerate case.
				// \\host.example.com (without a share name)
				// becomes
				// file://host.example.com/
				return &url.URL{
					Scheme: "file",
					Host:   path,
					Path:   "/",
				}, nil
			}

			// \\host.example.com\Share\path\to\file
			// becomes
			// file://host.example.com/Share/path/to/file
			return &url.URL{
				Scheme: "file",
				Host:   path[:i],
				Path:   filepath.ToSlash(path[i:]),
			}, nil
		}

		// C:\path\to\file
		// becomes
		// file:///C:/path/to/file
		return &url.URL{
			Scheme: "file",
			Path:   "/" + filepath.ToSlash(path),
		}, nil
	}

	// /path/to/file
	// becomes
	// file:///path/to/file
	return &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}, nil
}

func convertFileURLPath(host, path string) (string, error) {
	if runtime.GOOS == "windows" {
		return convertFileURLPathWindows(host, path)
	}
	switch host {
	case "", "localhost":
	default:
		return "", errors.New("file URL specifies non-local host")
	}
	return filepath.FromSlash(path), nil
}

func convertFileURLPathWindows(host, path string) (string, error) {
	if len(path) == 0 || path[0] != '/' {
		return "", errNotAbsolute
	}

	path = filepath.FromSlash(path)

	// We interpret Windows file URLs per the description in
	// https://blogs.msdn.microsoft.com/ie/2006/12/06/file-uris-in-windows/.

	// The host part of a file URL (if any) is the UNC volume name,
	// but RFC 8089 reserves the authority "localhost" for the local machine.
	if host != "" && host != "localhost" {
		// A common "legacy" format omits the leading slash before a drive letter,
		// encoding the drive letter as the host instead of part of the path.
		// (See https://blogs.msdn.microsoft.com/freeassociations/2005/05/19/the-bizarre-and-unhappy-story-of-file-urls/.)
		// We do not support that format, but we should at least emit a more
		// helpful error message for it.
		if filepath.VolumeName(host) != "" {
			return "", errors.New("file URL encodes volume in host field: too few slashes?")
		}
		return `\\` + host + path, nil
	}

	// If host is empty, path must contain an initial slash followed by a
	// drive letter and path. Remove the slash and verify that the path is valid.
	if vol := filepath.VolumeName(path[1:]); vol == "" || strings.HasPrefix(vol, `\\`) {
		return "", errors.New("file URL missing drive letter")
	}
	return path[1:], nil
}
