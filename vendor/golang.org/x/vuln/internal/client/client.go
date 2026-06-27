// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package client provides an interface for accessing vulnerability
// databases, via either HTTP or local filesystem access.
//
// The protocol is described at https://go.dev/security/vuln/database.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/vuln/internal/derrors"
	"golang.org/x/vuln/internal/osv"
	isem "golang.org/x/vuln/internal/semver"
	"golang.org/x/vuln/internal/web"
)

// A Client for reading vulnerability databases.
type Client struct {
	source
}

type Options struct {
	HTTPClient *http.Client
}

// NewClient returns a client that reads the vulnerability database
// in source (an "http" or "file" prefixed URL).
//
// It supports databases following the API described
// in https://go.dev/security/vuln/database#api.
func NewClient(source string, opts *Options) (_ *Client, err error) {
	source = strings.TrimRight(source, "/")
	uri, err := url.Parse(source)
	if err != nil {
		return nil, err
	}
	switch uri.Scheme {
	case "http", "https":
		return newHTTPClient(uri, opts)
	case "file":
		return newLocalClient(uri)
	default:
		return nil, fmt.Errorf("source %q has unsupported scheme", uri)
	}
}

var errUnknownSchema = errors.New("unrecognized vulndb format; see https://go.dev/security/vuln/database#api for accepted schema")

func newHTTPClient(uri *url.URL, opts *Options) (*Client, error) {
	source := uri.String()

	// v1 returns true if the source likely follows the V1 schema.
	v1 := func() bool {
		return source == "https://vuln.go.dev" ||
			endpointExistsHTTP(source, "index/modules.json.gz")
	}

	if v1() {
		return &Client{source: newHTTPSource(uri.String(), opts)}, nil
	}

	return nil, errUnknownSchema
}

func endpointExistsHTTP(source, endpoint string) bool {
	r, err := http.Head(source + "/" + endpoint)
	return err == nil && r.StatusCode == http.StatusOK
}

func newLocalClient(uri *url.URL) (*Client, error) {
	dir, err := toDir(uri)
	if err != nil {
		return nil, err
	}

	// Check if the DB likely follows the v1 schema by
	// looking for the "index/modules.json" endpoint.
	if endpointExistsDir(dir, modulesEndpoint+".json") {
		return &Client{source: newLocalSource(dir)}, nil
	}

	// If the DB doesn't follow the v1 schema,
	// attempt to intepret it as a flat list of OSV files.
	// This is currently a "hidden" feature, so don't output the
	// specific error if this fails.
	src, err := newHybridSource(dir)
	if err != nil {
		return nil, errUnknownSchema
	}
	return &Client{source: src}, nil
}

func toDir(uri *url.URL) (string, error) {
	dir, err := web.URLToFilePath(uri)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("%s is not a directory", dir)
	}
	return dir, nil
}

func endpointExistsDir(dir, endpoint string) bool {
	_, err := os.Stat(filepath.Join(dir, endpoint))
	return err == nil
}

func NewInMemoryClient(entries []*osv.Entry) (*Client, error) {
	s, err := newInMemorySource(entries)
	if err != nil {
		return nil, err
	}
	return &Client{source: s}, nil
}

func (c *Client) LastModifiedTime(ctx context.Context) (_ time.Time, err error) {
	derrors.Wrap(&err, "LastModifiedTime()")

	b, err := c.source.get(ctx, dbEndpoint)
	if err != nil {
		return time.Time{}, err
	}

	var dbMeta dbMeta
	if err := json.Unmarshal(b, &dbMeta); err != nil {
		return time.Time{}, err
	}

	return dbMeta.Modified, nil
}

type ModuleRequest struct {
	// The module path to filter on.
	// This must be set (if empty, ByModule errors).
	Path string
	// (Optional) If set, only return vulnerabilities affected
	// at this version.
	Version string
}

type ModuleResponse struct {
	Path    string
	Version string
	Entries []*osv.Entry
}

// ByModules returns a list of responses
// containing the OSV entries corresponding to each request.
//
// The order of the requests is preserved, and each request has
// a response even if there are no entries (in which case the Entries
// field is nil).
func (c *Client) ByModules(ctx context.Context, reqs []*ModuleRequest) (_ []*ModuleResponse, err error) {
	derrors.Wrap(&err, "ByModules(%v)", reqs)

	metas, err := c.moduleMetas(ctx, reqs)
	if err != nil {
		return nil, err
	}

	resps := make([]*ModuleResponse, len(reqs))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for i, req := range reqs {
		i, req := i, req
		g.Go(func() error {
			entries, err := c.byModule(gctx, req, metas[i])
			if err != nil {
				return err
			}
			resps[i] = &ModuleResponse{
				Path:    req.Path,
				Version: req.Version,
				Entries: entries,
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return resps, nil
}

func (c *Client) moduleMetas(ctx context.Context, reqs []*ModuleRequest) (_ []*moduleMeta, err error) {
	b, err := c.source.get(ctx, modulesEndpoint)
	if err != nil {
		return nil, err
	}

	dec, err := newStreamDecoder(b)
	if err != nil {
		return nil, err
	}

	metas := make([]*moduleMeta, len(reqs))
	for dec.More() {
		var m moduleMeta
		err := dec.Decode(&m)
		if err != nil {
			return nil, err
		}
		for i, req := range reqs {
			if m.Path == req.Path {
				metas[i] = &m
			}
		}
	}

	return metas, nil
}

// byModule returns the OSV entries matching the ModuleRequest,
// or (nil, nil) if there are none.
func (c *Client) byModule(ctx context.Context, req *ModuleRequest, m *moduleMeta) (_ []*osv.Entry, err error) {
	// This module isn't in the database.
	if m == nil {
		return nil, nil
	}

	if req.Path == "" {
		return nil, fmt.Errorf("module path must be set")
	}

	if req.Version != "" && !isem.Valid(req.Version) {
		return nil, fmt.Errorf("version %s is not valid semver", req.Version)
	}

	var ids []string
	for _, v := range m.Vulns {
		if v.Fixed == "" || isem.Less(req.Version, v.Fixed) {
			ids = append(ids, v.ID)
		}
	}

	if len(ids) == 0 {
		return nil, nil
	}

	entries, err := c.byIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	// Filter by version.
	if req.Version != "" {
		affected := func(e *osv.Entry) bool {
			for _, a := range e.Affected {
				if a.Module.Path == req.Path && isem.Affects(a.Ranges, req.Version) {
					return true
				}
			}
			return false
		}

		var filtered []*osv.Entry
		for _, entry := range entries {
			if affected(entry) {
				filtered = append(filtered, entry)
			}
		}
		if len(filtered) == 0 {
			return nil, nil
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	return entries, nil
}

func (c *Client) byIDs(ctx context.Context, ids []string) (_ []*osv.Entry, err error) {
	entries := make([]*osv.Entry, len(ids))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(10)
	for i, id := range ids {
		i, id := i, id
		g.Go(func() error {
			e, err := c.byID(gctx, id)
			if err != nil {
				return err
			}
			entries[i] = e
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return entries, nil
}

// byID returns the OSV entry with the given ID,
// or an error if it does not exist / cannot be unmarshaled.
func (c *Client) byID(ctx context.Context, id string) (_ *osv.Entry, err error) {
	derrors.Wrap(&err, "byID(%s)", id)

	b, err := c.source.get(ctx, entryEndpoint(id))
	if err != nil {
		return nil, err
	}

	var entry osv.Entry
	if err := json.Unmarshal(b, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// newStreamDecoder returns a decoder that can be used
// to read an array of JSON objects.
func newStreamDecoder(b []byte) (*json.Decoder, error) {
	dec := json.NewDecoder(bytes.NewBuffer(b))

	// skip open bracket
	_, err := dec.Token()
	if err != nil {
		return nil, err
	}

	return dec, nil
}
