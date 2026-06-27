package twwidth

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Tab rune

const (
	TabWidthDefault     = 8
	TabString       Tab = '\t'
)

// IsTab returns true if t equals the default tab.
func (t Tab) IsTab() bool {
	return t == TabString
}

func (t Tab) Byte() byte {
	return byte(t)
}

func (t Tab) Rune() rune {
	return rune(t)
}

func (t Tab) String() string {
	return string(t)
}

// IsTab returns true if r is a tab rune.
func IsTab(r rune) bool {
	return r == TabString.Rune()
}

type Tabinal struct {
	once  sync.Once
	width int
	mu    sync.RWMutex
}

func (t *Tabinal) String() string {
	return TabString.String()
}

// Size returns the current tab width, default if unset.
func (t *Tabinal) Size() int {
	t.once.Do(t.init)

	t.mu.RLock()
	w := t.width
	t.mu.RUnlock()

	if w <= 0 {
		return TabWidthDefault
	}
	return w
}

// SetWidth sets the tab width if valid (1–32).
func (t *Tabinal) SetWidth(w int) {
	if w <= 0 || w > 32 {
		return
	}
	t.mu.Lock()
	t.width = w
	t.mu.Unlock()
}

func (t *Tabinal) init() {
	w := t.detect()
	t.mu.Lock()
	t.width = w
	t.mu.Unlock()
}

// detect determines tab width using env, editorconfig, project, or term.
func (t *Tabinal) detect() int {
	if w := envInt("TABWIDTH"); w > 0 {
		return clamp(w)
	}
	if w := envInt("TS"); w > 0 {
		return clamp(w)
	}
	if w := envInt("VIM_TABSTOP"); w > 0 {
		return clamp(w)
	}
	if w := editorConfigTabWidth(); w > 0 {
		return w
	}
	if w := projectHeuristic(); w > 0 {
		return w
	}
	if w := termHeuristic(); w > 0 {
		return w
	}
	return 0
}

func editorConfigTabWidth() int {
	dir, err := os.Getwd()
	if err != nil {
		return 0
	}

	for dir != "" && dir != "/" && dir != "." {
		path := filepath.Join(dir, ".editorconfig")
		if w := parseEditorConfig(path); w > 0 {
			return clamp(w)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return 0
}

// parseEditorConfig reads tab_width or indent_size from a file.
func parseEditorConfig(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	inMatch := false
	globalWidth := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			pattern := line[1 : len(line)-1]
			inMatch = pattern == "*"

			knownExts := []string{".go", ".py", ".js", ".ts", ".java", ".rs"}
			for _, ext := range knownExts {
				if strings.Contains(pattern, ext) {
					inMatch = true
					break
				}
			}
			continue
		}

		if !inMatch && globalWidth == 0 {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "tab_width":
			if w, err := strconv.Atoi(val); err == nil && w > 0 {
				if inMatch {
					return clamp(w)
				}
				if globalWidth == 0 {
					globalWidth = w
				}
			}
		case "indent_size":
			if val == "tab" {
				continue
			}
			if w, err := strconv.Atoi(val); err == nil && w > 0 {
				if inMatch {
					return clamp(w)
				}
				if globalWidth == 0 {
					globalWidth = w
				}
			}
		}
	}

	return globalWidth
}

// projectHeuristic returns 4 for known project types.
func projectHeuristic() int {
	dir, err := os.Getwd()
	if err != nil {
		return 0
	}

	indicators := []string{
		"go.mod", "go.sum",
		"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"setup.py", "pyproject.toml", "requirements.txt", "Pipfile",
		"pom.xml", "build.gradle", "build.gradle.kts",
		"Cargo.toml",
		"composer.json",
	}

	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(dir, indicator)); err == nil {
			return 4
		}
	}

	patterns := []string{"*.go", "*.py", "*.js", "*.ts", "*.java", "*.rs"}
	for _, pattern := range patterns {
		if matches, _ := filepath.Glob(filepath.Join(dir, pattern)); len(matches) > 0 {
			return 4
		}
	}

	return 0
}

// termHeuristic returns a default width based on the TERM variable.
func termHeuristic() int {
	termEnv := strings.ToLower(os.Getenv("TERM"))
	if termEnv == "" {
		return 0
	}

	if strings.Contains(termEnv, "vt52") {
		return 2
	}

	if strings.Contains(termEnv, "xterm") ||
		strings.Contains(termEnv, "screen") ||
		strings.Contains(termEnv, "tmux") ||
		strings.Contains(termEnv, "linux") ||
		strings.Contains(termEnv, "ansi") ||
		strings.Contains(termEnv, "rxvt") {
		return TabWidthDefault
	}

	return 0
}

func clamp(w int) int {
	if w <= 0 {
		return 0
	}
	if w > 32 {
		return 32
	}
	return w
}

var (
	globalTab     *Tabinal
	globalTabOnce sync.Once
)

// TabInstance returns the singleton Tabinal.
func TabInstance() *Tabinal {
	globalTabOnce.Do(func() {
		globalTab = &Tabinal{}
	})
	return globalTab
}

// TabWidth returns the detected global tab width.
func TabWidth() int {
	return TabInstance().Size()
}

// SetTabWidth sets the global tab width.
func SetTabWidth(w int) {
	TabInstance().SetWidth(w)
}

func envInt(k string) int {
	v := os.Getenv(k)
	w, _ := strconv.Atoi(v)
	return w
}
