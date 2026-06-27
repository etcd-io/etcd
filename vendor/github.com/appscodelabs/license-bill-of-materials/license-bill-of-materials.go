package main

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	iofs "io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed assets/*.txt
var fs embed.FS

// Template holds pre-constructed license template info
type Template struct {
	Title    string
	Nickname string
	Words    map[string]int
}

func parseTemplate(content string) (*Template, error) {
	t := Template{}
	text := []byte{}
	state := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if state == 0 {
			if line == "---" {
				state = 1
			}
		} else if state == 1 {
			if line == "---" {
				state = 2
			} else {
				if strings.HasPrefix(line, "title:") {
					t.Title = strings.TrimSpace(line[len("title:"):])
				} else if strings.HasPrefix(line, "nickname:") {
					t.Nickname = strings.TrimSpace(line[len("nickname:"):])
				}
			}
		} else if state == 2 {
			text = append(text, scanner.Bytes()...)
			text = append(text, []byte("\n")...)
		}
	}
	t.Words = makeWordSet(text)
	return &t, scanner.Err()
}

func loadTemplates() ([]*Template, error) {
	templates := []*Template{}

	err := iofs.WalkDir(fs, "assets", func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		content, err := iofs.ReadFile(fs, path)
		if err != nil {
			return err
		}

		templ, err := parseTemplate(string(content))
		if err != nil {
			return err
		}
		templates = append(templates, templ)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return templates, nil
}

var (
	reWords     = regexp.MustCompile(`[\w']+`)
	reCopyright = regexp.MustCompile(
		`(?i)\s*Copyright (?:©|\(c\)|\xC2\xA9)?\s*(?:\d{4}|\[year\]).*`)
)

func cleanLicenseData(data []byte) []byte {
	data = bytes.ToLower(data)
	data = reCopyright.ReplaceAll(data, nil)
	return data
}

func makeWordSet(data []byte) map[string]int {
	words := map[string]int{}
	data = cleanLicenseData(data)
	matches := reWords.FindAll(data, -1)
	for i, m := range matches {
		s := string(m)
		if _, ok := words[s]; !ok {
			// Non-matching words are likely in the license header, to mention
			// copyrights and authors. Try to preserve the initial sequences,
			// to display them later.
			words[s] = i
		}
	}
	return words
}

// Word holds word and word position in a license
type Word struct {
	Text string
	Pos  int
}

type sortedWords []Word

func (s sortedWords) Len() int {
	return len(s)
}

func (s sortedWords) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortedWords) Less(i, j int) bool {
	return s[i].Pos < s[j].Pos
}

// MatchResult represents a matched template and matching metrics
type MatchResult struct {
	Template     *Template
	Score        float64
	ExtraWords   []string
	MissingWords []string
}

func sortAndReturnWords(words []Word) []string {
	sort.Sort(sortedWords(words))
	tokens := []string{}
	for _, w := range words {
		tokens = append(tokens, w.Text)
	}
	return tokens
}

// matchTemplates returns the best license template matching supplied data,
// its score between 0 and 1 and the list of words appearing in license but not
// in the matched template.
func matchTemplates(license []byte, templates []*Template) MatchResult {
	bestScore := float64(-1)
	var bestTemplate *Template
	bestExtra := []Word{}
	bestMissing := []Word{}
	words := makeWordSet(license)
	for _, t := range templates {
		extra := []Word{}
		missing := []Word{}
		common := 0
		for w, pos := range words {
			_, ok := t.Words[w]
			if ok {
				common++
			} else {
				extra = append(extra, Word{
					Text: w,
					Pos:  pos,
				})
			}
		}
		for w, pos := range t.Words {
			if _, ok := words[w]; !ok {
				missing = append(missing, Word{
					Text: w,
					Pos:  pos,
				})
			}
		}
		score := 2 * float64(common) / (float64(len(words)) + float64(len(t.Words)))
		if score > bestScore {
			bestScore = score
			bestTemplate = t
			bestMissing = missing
			bestExtra = extra
		}
	}
	return MatchResult{
		Template:     bestTemplate,
		Score:        bestScore,
		ExtraWords:   sortAndReturnWords(bestExtra),
		MissingWords: sortAndReturnWords(bestMissing),
	}
}

// fixEnv returns a copy of the process environment where GOPATH is adjusted to
// supplied value. It returns nil if gopath is empty.
func fixEnv(gopath string) []string {
	if gopath == "" {
		return nil
	}
	kept := []string{
		"GOPATH=" + gopath,
	}
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "GOPATH=") {
			kept = append(kept, env)
		}
	}
	return kept
}

// expandPackages takes a list of package or package expressions and invoke go
// list to expand them to packages. In particular, it handles things like "..."
// and ".".
func expandPackages(gopath string, pkgs []string) ([]string, error) {
	args := []string{"list"}
	args = append(args, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Env = fixEnv(gopath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("'go %s' failed with:\n%s",
			strings.Join(args, " "), string(out))
	}
	names := []string{}
	for _, s := range strings.Split(string(out), "\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			names = append(names, s)
		}
	}
	return names, nil
}

func listPackagesAndDeps(gopath string, pkgs []string) ([]string, error) {
	pkgs, err := expandPackages(gopath, pkgs)
	if err != nil {
		return nil, err
	}
	args := []string{"list", "-f", "{{range .Deps}}{{.}}|{{end}}"}
	args = append(args, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Env = fixEnv(gopath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("'go %s' failed with:\n%s",
			strings.Join(args, " "), string(out))
	}
	deps := []string{}
	seen := map[string]bool{}
	for _, s := range strings.Split(string(out), "|") {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			deps = append(deps, s)
			seen[s] = true
		}
	}
	for _, pkg := range pkgs {
		if !seen[pkg] {
			seen[pkg] = true
			deps = append(deps, pkg)
		}
	}
	sort.Strings(deps)
	return deps, nil
}

func listStandardPackages(gopath string) ([]string, error) {
	return expandPackages(gopath, []string{"std", "cmd"})
}

// PkgError reports on missing packages
type PkgError struct {
	Err string
}

// PkgInfo holds identifying package info
type PkgInfo struct {
	Name       string
	Dir        string
	Root       string
	ImportPath string
	Error      *PkgError
}

func getPackagesInfo(gopath string, pkgs []string) ([]*PkgInfo, error) {
	args := []string{"list", "-e", "-json"}
	// TODO: split the list for platforms which do not support massive argument
	// lists.
	args = append(args, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Env = fixEnv(gopath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go %s failed with:\n%s",
			strings.Join(args, " "), string(out))
	}
	infos := make([]*PkgInfo, 0, len(pkgs))
	decoder := json.NewDecoder(bytes.NewBuffer(out))
	var derr error
	for _, pkg := range pkgs {
		info := &PkgInfo{}
		derr = decoder.Decode(info)
		if derr != nil {
			return nil, fmt.Errorf("could not retrieve package information for %s", pkg)
		}
		if pkg != info.ImportPath {
			return nil, fmt.Errorf("package information mismatch: asked for %s, got %s",
				pkg, info.ImportPath)
		}
		if info.Error != nil && info.Name == "" {
			info.Name = info.ImportPath
		}
		infos = append(infos, info)
	}
	return infos, err
}

var (
	reLicense = regexp.MustCompile(`(?i)^(?:` +
		`((?:un)?licen[sc]e(?:\.[^.]+)?)|` +
		`(copy(?:ing|right)(?:\.[^.]+)?)|` +
		`)$`)
)

// scoreLicenseName returns a factor between 0 and 1 weighting how likely
// supplied filename is a license file.
func scoreLicenseName(name string) int8 {
	m := reLicense.FindStringSubmatch(name)
	switch {
	case m == nil:
		break
	case m[1] != "" || m[2] != "":
		return 1
	}
	return 0
}

// findLicenses looks for license files in package import path, and down to
// parent directories until a file is found or $GOPATH/src is reached. It
// returns a slice of paths all viable files, or a slice containing one empty
// string if none were found.
func findLicenses(info *PkgInfo) ([]string, error) {
	path := info.Dir
	for ; path != filepath.Dir(info.Root); path = filepath.Dir(path) {
		fis, err := ioutil.ReadDir(path)
		if err != nil {
			return []string{""}, err
		}
		allViableNames := make([]string, 0)
		for _, fi := range fis {
			if !fi.Mode().IsRegular() {
				continue
			}
			score := scoreLicenseName(fi.Name())
			if score == 1 {
				allViableNames = append(allViableNames, filepath.Join(path, fi.Name()))
			}
		}
		if len(allViableNames) > 0 {
			return allViableNames, nil
		}
	}
	return []string{""}, nil
}

// GoPackage represents a top-level package, ex. colors/blue
type GoPackage struct {
	PackageName string
	RawLicenses []*RawLicense
	Err         string
}

// RawLicense holds template-matched file data
type RawLicense struct {
	Path         string
	Score        float64
	Template     *Template
	ExtraWords   []string
	MissingWords []string
}

func listPackagesWithLicenses(gopath string, pkgs []string) ([]GoPackage, error) {
	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}
	deps, err := listPackagesAndDeps(gopath, pkgs)
	if err != nil {
		return nil, fmt.Errorf("could not list %s dependencies: %s",
			strings.Join(pkgs, " "), err)
	}
	std, err := listStandardPackages(gopath)
	if err != nil {
		return nil, fmt.Errorf("could not list standard packages: %s", err)
	}
	stdSet := map[string]bool{}
	for _, n := range std {
		stdSet[n] = true
	}
	infos, err := getPackagesInfo(gopath, deps)
	if err != nil {
		return nil, err
	}

	// Cache matched licenses by path. Useful for package with a lot of
	// subpackages like bleve.
	matched := map[string]MatchResult{}

	gPackages := []GoPackage{}
	for _, info := range infos {
		if info.Error != nil {
			gPackages = append(gPackages, GoPackage{
				PackageName: info.Name,
				Err:         info.Error.Err,
				RawLicenses: []*RawLicense{{Path: ""}},
			})
			continue
		}
		if stdSet[info.ImportPath] {
			continue
		}
		paths, err := findLicenses(info)
		if err != nil {
			return nil, err
		}
		rawLicenseInfos := []*RawLicense{}
		gPackage := GoPackage{PackageName: info.ImportPath}
		for _, path := range paths {
			rl := RawLicense{Path: path}
			if path != "" {
				m, ok := matched[path]
				if !ok {
					data, err := ioutil.ReadFile(path)
					if err != nil {
						return nil, err
					}
					m = matchTemplates(data, templates)
					matched[path] = m
				}
				rl.Score = m.Score
				rl.Template = m.Template
				rl.ExtraWords = m.ExtraWords
				rl.MissingWords = m.MissingWords
			}
			rawLicenseInfos = append(rawLicenseInfos, &rl)
		}
		gPackage.RawLicenses = rawLicenseInfos
		gPackages = append(gPackages, gPackage)
	}
	return gPackages, nil
}

// longestCommonPrefix returns the longest common prefix over import path
// components of supplied licenses.
func longestCommonPrefix(gPackages []GoPackage) string {
	type Node struct {
		Name     string
		Children map[string]*Node
		Shared   int
	}
	// Build a prefix tree. Not super efficient, but easy to do.
	root := &Node{
		Children: map[string]*Node{},
		Shared:   len(gPackages),
	}
	for _, l := range gPackages {
		n := root
		for _, part := range strings.Split(l.PackageName, "/") {
			c := n.Children[part]
			if c == nil {
				c = &Node{
					Name:     part,
					Children: map[string]*Node{},
				}
				n.Children[part] = c
			}
			c.Shared++
			n = c
		}
	}
	n := root
	prefix := []string{}
	for {
		if len(n.Children) != 1 {
			break
		}
		for _, c := range n.Children {
			if c.Shared == len(gPackages) {
				// Handle case where there are subpackages:
				// prometheus/procfs
				// prometheus/procfs/xfs
				prefix = append(prefix, c.Name)
			}
			n = c
			break
		}
	}
	return strings.Join(prefix, "/")
}

// groupPackagesByLicense returns the input packages after grouping them by license
// path and find their longest import path common prefix. Entries with empty
// paths are left unchanged.
func groupPackagesByLicense(gPackages []GoPackage) ([]GoPackage, error) {
	paths := map[string][]GoPackage{}
	for _, gp := range gPackages {
		for _, rl := range gp.RawLicenses {
			if rl.Path == "" {
				continue
			}
			paths[rl.Path] = append(paths[rl.Path], gp)
		}
	}
	for k, v := range paths {
		if len(v) <= 1 {
			continue
		}
		prefix := longestCommonPrefix(v)
		if prefix == "" {
			// return nil, fmt.Errorf(
			// 	"packages share the same license but not common prefix: %v", v)
			fmt.Fprintf(os.Stderr, "packages share the same license but not common prefix: %v\n", v)
		}
		gp := v[0]
		gp.PackageName = prefix
		paths[k] = []GoPackage{gp}
	}
	kept := []GoPackage{}
	// Ensures only one package with multiple licenses is appended to the list of
	// kept packages
	seen := make(map[string]bool)
	for _, gp := range gPackages {
		if len(gp.RawLicenses) == 0 {
			kept = append(kept, gp)
			continue
		}
		for _, rl := range gp.RawLicenses {
			if rl.Path == "" {
				kept = append(kept, gp)
				continue
			}
			if v, ok := paths[rl.Path]; ok {
				if _, ok := seen[v[0].PackageName]; !ok {
					kept = append(kept, v[0])
					delete(paths, rl.Path)
					seen[v[0].PackageName] = true
				}
			}
		}
	}
	return kept, nil
}

type projectAndLicenses struct {
	Project  string    `json:"project"`
	Licenses []license `json:"licenses,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type license struct {
	Type       string  `json:"type,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

func licensesToProjectAndLicenses(gPackages []GoPackage) (c []projectAndLicenses, e []projectAndLicenses) {
	for _, gp := range gPackages {
		if gp.Err != "" {
			e = append(e, projectAndLicenses{
				Project: removeVendor(gp.PackageName),
				Error:   gp.Err,
			})
			continue
		}
		nt := 0
		for _, rl := range gp.RawLicenses {
			if rl.Template == nil {
				nt++
			}
		}
		if len(gp.RawLicenses) == nt {
			e = append(e, projectAndLicenses{
				Project: removeVendor(gp.PackageName),
				Error:   "No license detected",
			})
			continue
		}
		ls := []license{}
		for _, rl := range gp.RawLicenses {
			if rl.Template.Title != "" {
				ls = append(ls, license{
					Type:       rl.Template.Title,
					Confidence: rl.Score,
				})
			}
		}
		c = append(c, projectAndLicenses{
			Project:  removeVendor(gp.PackageName),
			Licenses: ls,
		})
	}
	return c, e
}

func removeVendor(s string) string {
	v := "/vendor/"
	i := strings.Index(s, v)
	if i == -1 {
		return s
	}
	return s[i+len(v):]
}

func truncateFloat(f float64) float64 {
	nf := fmt.Sprintf("%.3f", f)

	var err error
	f, err = strconv.ParseFloat(nf, 64)
	if err != nil {
		panic("unexpected parse float error")
	}
	return f
}

func pkgsToLicenses(pkgs []string, overrides string) (pls []projectAndLicenses, ne []projectAndLicenses) {
	fplm := make(map[string][]string)
	if err := json.Unmarshal([]byte(overrides), &pls); err != nil {
		log.Fatal(err)
	}
	for _, pl := range pls {
		for _, l := range pl.Licenses {
			fplm[pl.Project] = append(fplm[pl.Project], l.Type)
		}
	}

	licenses, err := listPackagesWithLicenses("", pkgs)
	if err != nil {
		log.Fatal(err)
	}
	if licenses, err = groupPackagesByLicense(licenses); err != nil {
		log.Fatal(err)
	}
	c, e := licensesToProjectAndLicenses(licenses)

	// detected licenses
	pls = nil
	ls := []license{}
	for _, pl := range c {
		if fl, ok := fplm[pl.Project]; ok {
			for _, l := range fl {
				ls = append(ls, license{
					Type:       l,
					Confidence: 1.0,
				})
			}
			pl = projectAndLicenses{
				Project:  pl.Project,
				Licenses: ls,
			}
			delete(fplm, pl.Project)
		}
		pls = append(pls, pl)
	}
	// force add undetected licenses given by overrides
	ls = nil
	for proj, fl := range fplm {
		for _, l := range fl {
			ls = append(ls, license{
				Type:       l,
				Confidence: 1.0,
			})
		}
		pls = append(pls, projectAndLicenses{
			Project:  proj,
			Licenses: ls,
		})
	}
	// missing / error license
	for _, pl := range e {
		if _, ok := fplm[pl.Project]; !ok {
			ne = append(ne, pl)
		}
	}

	sort.Slice(pls, func(i, j int) bool { return pls[i].Project < pls[j].Project })
	sort.Slice(ne, func(i, j int) bool { return ne[i].Project < ne[j].Project })
	return pls, ne
}

func main() {
	of := flag.String("override-file", "", "a file to overwrite licenses")
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatal("expect at least one package argument")
	}

	overrides := "[]"
	if len(*of) != 0 {
		b, err := ioutil.ReadFile(*of)
		if err != nil {
			log.Fatal(err)
		}
		overrides = string(b)
	}

	c, ne := pkgsToLicenses(flag.Args(), overrides)
	b, err := json.MarshalIndent(c, "", "	")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))

	if len(ne) != 0 {
		fmt.Println("")
		b, err := json.MarshalIndent(ne, "", "	")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(b))
		os.Exit(1)
	}
}
