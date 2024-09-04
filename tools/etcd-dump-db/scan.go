// Copyright 2024 The etcd Authors
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

package main

import (
	"fmt"
	"io"
	"os"

	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

func scanKeys(dbPath string, startRev int64) error {
	pgSize, hwm, err := readPageAndHWMSize(dbPath)
	if err != nil {
		return fmt.Errorf("failed to read page and HWM size: %w", err)
	}

	for pageID := uint64(2); pageID < hwm; {
		p, _, err := readPage(dbPath, pgSize, pageID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Reading page %d failed: %v. Continuting...\n", pageID, err)
			pageID++
			continue
		}

		if !p.isLeafPage() {
			pageID++
			continue
		}

		for i := uint16(0); i < p.count; i++ {
			e := p.leafPageElement(i)

			rev, err := bytesToBucketKey(e.key())
			if err != nil {
				if exceptionCheck(e.key()) {
					break
				}
				fmt.Fprintf(os.Stderr, "Decoding revision failed, pageID: %d, index: %d, key: %x, error: %v\n", pageID, i, string(e.key()), err)
				continue
			}

			if startRev != 0 && rev.Main < startRev {
				continue
			}

			fmt.Printf("pageID=%d, index=%d/%d, ", pageID, i, p.count-1)
			keyDecoder(e.key(), e.value())
		}

		pageID += uint64(p.overflow) + 1
	}
	return nil
}

func bytesToBucketKey(key []byte) (rev mvcc.BucketKey, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("BytesToBucketKey failed: %v", r)
		}
	}()
	rev = mvcc.BytesToBucketKey(key)
	return rev, err
}

func readPageAndHWMSize(dbPath string) (uint64, uint64, error) {
	f, err := os.Open(dbPath)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	// read 4KB chunk
	buf := make([]byte, 4096)
	if _, err := io.ReadFull(f, buf); err != nil {
		return 0, 0, err
	}

	m := loadPageMeta(buf)
	if m.magic != magic {
		return 0, 0, fmt.Errorf("the Meta Page has wrong (unexpected) magic")
	}

	return uint64(m.pageSize), m.pgid, nil
}

func readPage(dbPath string, pageSize uint64, pageID uint64) (*page, []byte, error) {
	f, err := os.Open(dbPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	buf := make([]byte, pageSize)
	if _, err := f.ReadAt(buf, int64(pageID*pageSize)); err != nil {
		return nil, nil, err
	}

	p := loadPage(buf)
	if p.id != pageID {
		return nil, nil, fmt.Errorf("unexpected page id: %d, wanted: %d", p.id, pageID)
	}

	if p.overflow == 0 {
		return p, buf, nil
	}

	buf = make([]byte, (uint64(p.overflow)+1)*pageSize)
	if _, err := f.ReadAt(buf, int64(pageID*pageSize)); err != nil {
		return nil, nil, err
	}

	p = loadPage(buf)
	if p.id != pageID {
		return nil, nil, fmt.Errorf("unexpected page id: %d, wanted: %d", p.id, pageID)
	}

	return p, buf, nil
}

func exceptionCheck(key []byte) bool {
	whiteKeyList := map[string]struct{}{
		"alarm":           {},
		"auth":            {},
		"authRoles":       {},
		"authUsers":       {},
		"cluster":         {},
		"key":             {},
		"lease":           {},
		"members":         {},
		"members_removed": {},
		"meta":            {},
	}

	_, ok := whiteKeyList[string(key)]
	return ok
}
