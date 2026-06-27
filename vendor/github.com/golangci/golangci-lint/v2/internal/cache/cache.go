package cache

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/internal/go/cache"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/timeutils"
)

type HashMode int

const (
	HashModeNeedOnlySelf HashMode = iota
	HashModeNeedDirectDeps
	HashModeNeedAllDeps
)

var ErrMissing = errors.New("missing data")

type hashResults map[HashMode]string

// Cache is a per-package data cache.
// A cached data is invalidated when package,
// or it's dependencies change.
type Cache struct {
	lowLevelCache cache.Cache
	pkgHashes     sync.Map
	sw            *timeutils.Stopwatch
	log           logutils.Log
	ioSem         chan struct{} // semaphore limiting parallel IO
}

func NewCache(sw *timeutils.Stopwatch, log logutils.Log) (*Cache, error) {
	return &Cache{
		lowLevelCache: cache.Default(),
		sw:            sw,
		log:           log,
		ioSem:         make(chan struct{}, runtime.GOMAXPROCS(-1)),
	}, nil
}

func (c *Cache) Close() {
	err := c.sw.TrackStageErr("close", c.lowLevelCache.Close)
	if err != nil {
		c.log.Errorf("cache close: %v", err)
	}
}

func (c *Cache) Put(pkg *packages.Package, mode HashMode, key string, data any) error {
	buf, err := c.encode(data)
	if err != nil {
		return err
	}

	actionID, err := c.buildKey(pkg, mode, key)
	if err != nil {
		return fmt.Errorf("failed to calculate package %s action id: %w", pkg.Name, err)
	}

	err = c.putBytes(actionID, buf)
	if err != nil {
		return fmt.Errorf("failed to save data to low-level cache by key %s for package %s: %w", key, pkg.Name, err)
	}

	return nil
}

func (c *Cache) Get(pkg *packages.Package, mode HashMode, key string, data any) error {
	actionID, err := c.buildKey(pkg, mode, key)
	if err != nil {
		return fmt.Errorf("failed to calculate package %s action id: %w", pkg.Name, err)
	}

	cachedData, err := c.getBytes(actionID)
	if err != nil {
		if cache.IsErrMissing(err) {
			return ErrMissing
		}
		return fmt.Errorf("failed to get data from low-level cache by key %s for package %s: %w", key, pkg.Name, err)
	}

	return c.decode(cachedData, data)
}

func (c *Cache) buildKey(pkg *packages.Package, mode HashMode, key string) (cache.ActionID, error) {
	return timeutils.TrackStage(c.sw, "key build", func() (cache.ActionID, error) {
		actionID, err := c.pkgActionID(pkg, mode)
		if err != nil {
			return actionID, err
		}

		subkey, subkeyErr := cache.Subkey(actionID, key)
		if subkeyErr != nil {
			return actionID, fmt.Errorf("failed to build subkey: %w", subkeyErr)
		}

		return subkey, nil
	})
}

func (c *Cache) pkgActionID(pkg *packages.Package, mode HashMode) (cache.ActionID, error) {
	hash, err := c.packageHash(pkg, mode)
	if err != nil {
		return cache.ActionID{}, fmt.Errorf("failed to get package hash: %w", err)
	}

	key, err := cache.NewHash("action ID")
	if err != nil {
		return cache.ActionID{}, fmt.Errorf("failed to make a hash: %w", err)
	}

	fmt.Fprintf(key, "pkgpath %s\n", pkg.PkgPath)
	fmt.Fprintf(key, "pkghash %s\n", hash)

	return key.Sum(), nil
}

func (c *Cache) packageHash(pkg *packages.Package, mode HashMode) (string, error) {
	results, found := c.pkgHashes.Load(pkg)
	if found {
		hashRes := results.(hashResults)
		if result, ok := hashRes[mode]; ok {
			return result, nil
		}

		return "", fmt.Errorf("no mode %d in hash result", mode)
	}

	hashRes, err := c.computePkgHash(pkg)
	if err != nil {
		return "", err
	}

	result, found := hashRes[mode]
	if !found {
		return "", fmt.Errorf("invalid mode %d", mode)
	}

	c.pkgHashes.Store(pkg, hashRes)

	return result, nil
}

// computePkgHash computes a package's hash.
// The hash is based on all Go files that make up the package,
// as well as the hashes of imported packages.
func (c *Cache) computePkgHash(pkg *packages.Package) (hashResults, error) {
	key, err := cache.NewHash("package hash")
	if err != nil {
		return nil, fmt.Errorf("failed to make a hash: %w", err)
	}

	hashRes := hashResults{}

	fmt.Fprintf(key, "pkgpath %s\n", pkg.PkgPath)

	for _, f := range slices.Concat(pkg.CompiledGoFiles, pkg.IgnoredFiles) {
		h, fErr := c.fileHash(f)
		if fErr != nil {
			return nil, fmt.Errorf("failed to calculate file %s hash: %w", f, fErr)
		}

		// This is the current module (the project to analyze).
		if pkg.Module != nil && pkg.Module.Version == "" {
			f = pkg.Module.Path + strings.TrimPrefix(filepath.ToSlash(f), filepath.ToSlash(pkg.Module.Dir))
		}

		fmt.Fprintf(key, "file %s %x\n", f, h)
	}

	curSum := key.Sum()
	hashRes[HashModeNeedOnlySelf] = hex.EncodeToString(curSum[:])

	imps := slices.SortedFunc(maps.Values(pkg.Imports), func(a, b *packages.Package) int {
		return strings.Compare(a.PkgPath, b.PkgPath)
	})

	if err := c.computeDepsHash(HashModeNeedOnlySelf, imps, key); err != nil {
		return nil, err
	}

	curSum = key.Sum()
	hashRes[HashModeNeedDirectDeps] = hex.EncodeToString(curSum[:])

	if err := c.computeDepsHash(HashModeNeedAllDeps, imps, key); err != nil {
		return nil, err
	}

	curSum = key.Sum()
	hashRes[HashModeNeedAllDeps] = hex.EncodeToString(curSum[:])

	return hashRes, nil
}

func (c *Cache) computeDepsHash(depMode HashMode, imps []*packages.Package, key *cache.Hash) error {
	for _, dep := range imps {
		if dep.PkgPath == "unsafe" {
			continue
		}

		depHash, err := c.packageHash(dep, depMode)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for dependency %s with mode %d: %w", dep.Name, depMode, err)
		}

		fmt.Fprintf(key, "import %s %s\n", dep.PkgPath, depHash)
	}

	return nil
}

func (c *Cache) putBytes(actionID cache.ActionID, buf *bytes.Buffer) error {
	c.ioSem <- struct{}{}

	err := c.sw.TrackStageErr("cache io", func() error {
		return cache.PutBytes(c.lowLevelCache, actionID, buf.Bytes())
	})

	<-c.ioSem

	if err != nil {
		return err
	}

	return nil
}

func (c *Cache) getBytes(actionID cache.ActionID) ([]byte, error) {
	c.ioSem <- struct{}{}

	cachedData, err := timeutils.TrackStage(c.sw, "cache io", func() ([]byte, error) {
		b, _, errGB := cache.GetBytes(c.lowLevelCache, actionID)
		return b, errGB
	})

	<-c.ioSem

	if err != nil {
		return nil, err
	}

	return cachedData, nil
}

func (c *Cache) fileHash(f string) ([cache.HashSize]byte, error) {
	c.ioSem <- struct{}{}

	h, err := cache.FileHash(f)

	<-c.ioSem

	if err != nil {
		return [cache.HashSize]byte{}, err
	}

	return h, nil
}

func (c *Cache) encode(data any) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	err := c.sw.TrackStageErr("gob", func() error {
		return gob.NewEncoder(buf).Encode(data)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to gob encode: %w", err)
	}

	return buf, nil
}

func (c *Cache) decode(b []byte, data any) error {
	err := c.sw.TrackStageErr("gob", func() error {
		return gob.NewDecoder(bytes.NewReader(b)).Decode(data)
	})
	if err != nil {
		return fmt.Errorf("failed to gob decode: %w", err)
	}

	return nil
}

func SetSalt(b *bytes.Buffer) {
	cache.SetSalt(b.Bytes())
}

func DefaultDir() string {
	cacheDir, _ := cache.DefaultDir()
	return cacheDir
}
