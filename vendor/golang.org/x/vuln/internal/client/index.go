// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/vuln/internal/osv"
	isem "golang.org/x/vuln/internal/semver"
)

// indexFromDir returns a raw index created from a directory
// containing OSV entries.
// It skips any non-JSON files but errors if any of the JSON files
// cannot be unmarshaled into OSV, or have a filename other than <ID>.json.
func indexFromDir(dir string) (map[string][]byte, error) {
	idx := newIndex()
	f := os.DirFS(dir)

	if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		fname := d.Name()
		ext := filepath.Ext(fname)
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		case ext != ".json":
			return nil
		}

		b, err := fs.ReadFile(f, d.Name())
		if err != nil {
			return err
		}
		var entry osv.Entry
		if err := json.Unmarshal(b, &entry); err != nil {
			return err
		}
		if fname != entry.ID+".json" {
			return fmt.Errorf("OSV entries must have filename of the form <ID>.json, got %s", fname)
		}

		idx.add(&entry)
		return nil
	}); err != nil {
		return nil, err
	}

	return idx.raw()
}

func indexFromEntries(entries []*osv.Entry) (map[string][]byte, error) {
	idx := newIndex()

	for _, entry := range entries {
		idx.add(entry)
	}

	return idx.raw()
}

type index struct {
	db      *dbMeta
	modules modulesIndex
}

func newIndex() *index {
	return &index{
		db:      &dbMeta{},
		modules: make(map[string]*moduleMeta),
	}
}

func (i *index) add(entry *osv.Entry) {
	// Add to db index.
	if entry.Modified.After(i.db.Modified) {
		i.db.Modified = entry.Modified
	}
	// Add to modules index.
	for _, affected := range entry.Affected {
		modulePath := affected.Module.Path
		if _, ok := i.modules[modulePath]; !ok {
			i.modules[modulePath] = &moduleMeta{
				Path:  modulePath,
				Vulns: []moduleVuln{},
			}
		}
		module := i.modules[modulePath]
		module.Vulns = append(module.Vulns, moduleVuln{
			ID:       entry.ID,
			Modified: entry.Modified,
			Fixed:    isem.NonSupersededFix(affected.Ranges),
		})
	}
}

func (i *index) raw() (map[string][]byte, error) {
	data := make(map[string][]byte)

	b, err := json.Marshal(i.db)
	if err != nil {
		return nil, err
	}
	data[dbEndpoint] = b

	b, err = json.Marshal(i.modules)
	if err != nil {
		return nil, err
	}
	data[modulesEndpoint] = b

	return data, nil
}
