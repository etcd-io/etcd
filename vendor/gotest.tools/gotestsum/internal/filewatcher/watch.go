//go:build !aix
// +build !aix

package filewatcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"gotest.tools/gotestsum/internal/log"
)

const maxDepth = 7

type Event struct {
	// PkgPath of the package that triggered the event.
	PkgPath string
	// Args will be appended to the command line args for 'go test'.
	Args []string
	// Debug runs the tests with delve.
	Debug bool
	// resume the Watch goroutine when this channel is closed. Used to block
	// the Watch goroutine while tests are running.
	resume chan struct{}
	// reloadPaths will cause the watched path list to be reloaded, to watch
	// new directories.
	reloadPaths bool
	// useLastPath when true will use the PkgPath from the previous run.
	useLastPath bool
}

// Watch dirs for filesystem events, and run tests when .go files are saved.
//
//nolint:gocyclo
func Watch(ctx context.Context, dirs []string, clearScreen bool, run func(Event) error) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close() //nolint:errcheck // always returns nil error

	if err := loadPaths(watcher, dirs); err != nil {
		return err
	}

	timer := time.NewTimer(maxIdleTime)
	defer timer.Stop()

	term := newTerminal()
	defer term.Reset()
	go term.Monitor(ctx)

	h := &fsEventHandler{
		last:        time.Now(),
		clearScreen: clearScreen,
		fn:          run,
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return fmt.Errorf("exceeded idle timeout while watching files")

		case event := <-term.Events():
			resetTimer(timer)

			if event.reloadPaths {
				if err := loadPaths(watcher, dirs); err != nil {
					return err
				}
				close(event.resume)
				continue
			}

			term.Reset()
			if err := h.runTests(event); err != nil {
				return fmt.Errorf("failed to rerun tests for %v: %v", event.PkgPath, err)
			}
			term.Start()
			close(event.resume)

		case event := <-watcher.Events:
			resetTimer(timer)
			log.Debugf("handling event %v", event)

			if handleDirCreated(watcher, event) {
				continue
			}

			if err := h.handleEvent(event); err != nil {
				return fmt.Errorf("failed to run tests for %v: %v", event.Name, err)
			}

		case err := <-watcher.Errors:
			return fmt.Errorf("failed while watching files: %v", err)
		}
	}
}

const maxIdleTime = time.Hour

func resetTimer(timer *time.Timer) {
	if !timer.Stop() {
		<-timer.C
	}
	timer.Reset(maxIdleTime)
}

func loadPaths(watcher *fsnotify.Watcher, dirs []string) error {
	toWatch := findAllDirs(dirs, maxDepth)
	fmt.Printf("Watching %v directories. Use Ctrl-c to stop a run or exit.\n", len(toWatch))
	for _, dir := range toWatch {
		if err := watcher.Add(dir); err != nil {
			return fmt.Errorf("failed to watch directory %v: %w", dir, err)
		}
	}
	return nil
}

func findAllDirs(dirs []string, maxDepth int) []string {
	if len(dirs) == 0 {
		dirs = []string{"./..."}
	}

	var output []string //nolint:prealloc
	for _, dir := range dirs {
		const recur = "/..."
		if strings.HasSuffix(dir, recur) {
			dir = strings.TrimSuffix(dir, recur)
			output = append(output, findSubDirs(dir, maxDepth)...)
			continue
		}
		output = append(output, dir)
	}
	return output
}

func findSubDirs(rootDir string, maxDepth int) []string {
	var output []string
	// add root dir depth so that maxDepth is relative to the root dir
	maxDepth += pathDepth(rootDir)
	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warnf("failed to watch %v: %v", path, err)
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if pathDepth(path) > maxDepth || exclude(path) {
			log.Debugf("Ignoring %v because of max depth or exclude list", path)
			return filepath.SkipDir
		}
		if !hasGoFiles(path) {
			log.Debugf("Ignoring %v because it has no .go files", path)
			return nil
		}
		output = append(output, path)
		return nil
	}
	//nolint:errcheck // error is handled by walker func
	filepath.Walk(rootDir, walker)
	return output
}

func pathDepth(path string) int {
	return strings.Count(filepath.Clean(path), string(filepath.Separator))
}

// return true if path is vendor, testdata, or starts with a dot
func exclude(path string) bool {
	base := filepath.Base(path)
	switch {
	case strings.HasPrefix(base, ".") && len(base) > 1:
		return true
	case base == "vendor" || base == "testdata":
		return true
	}
	return false
}

func hasGoFiles(path string) bool {
	fh, err := os.Open(path)
	if err != nil {
		return false
	}
	defer fh.Close() //nolint:errcheck // fh is opened read-only

	for {
		names, err := fh.Readdirnames(20)
		switch {
		case err == io.EOF:
			return false
		case err != nil:
			log.Warnf("failed to read directory %v: %v", path, err)
			return false
		}

		for _, name := range names {
			if strings.HasSuffix(name, ".go") {
				return true
			}
		}
	}
}

func handleDirCreated(watcher *fsnotify.Watcher, event fsnotify.Event) (handled bool) {
	if event.Op&fsnotify.Create != fsnotify.Create {
		return false
	}

	fileInfo, err := os.Stat(event.Name)
	if err != nil {
		log.Debugf("failed to stat %s: %s", event.Name, err)
		return false
	}

	if !fileInfo.IsDir() {
		return false
	}

	if err := watcher.Add(event.Name); err != nil {
		log.Warnf("failed to watch new directory %v: %v", event.Name, err)
	}
	return true
}

type fsEventHandler struct {
	last        time.Time
	lastPath    string
	clearScreen bool
	fn          func(opts Event) error
}

var floodThreshold = 250 * time.Millisecond

func (h *fsEventHandler) handleEvent(event fsnotify.Event) error {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
		return nil
	}

	if !strings.HasSuffix(event.Name, ".go") {
		return nil
	}

	if time.Since(h.last) < floodThreshold {
		log.Debugf("skipping event received less than %v after the previous", floodThreshold)
		return nil
	}
	return h.runTests(Event{PkgPath: "./" + filepath.Dir(event.Name)})
}

func (h *fsEventHandler) runTests(opts Event) error {
	if opts.useLastPath {
		opts.PkgPath = h.lastPath
	}

	if h.clearScreen {
		fmt.Println("\033[H\033[2J")
	}

	fmt.Printf("\nRunning tests in %v\n", opts.PkgPath)

	if err := h.fn(opts); err != nil {
		return err
	}
	h.last = time.Now()
	h.lastPath = opts.PkgPath
	return nil
}
