package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/fatih/camelcase"
	"github.com/fatih/structtag"
	"golang.org/x/tools/go/buildutil"
)

// structType contains a structType node and it's name. It's a convenient
// helper type, because *ast.StructType doesn't contain the name of the struct
type structType struct {
	name string
	node *ast.StructType
}

// output is used usually by editors
type output struct {
	Start  int      `json:"start"`
	End    int      `json:"end"`
	Lines  []string `json:"lines"`
	Errors []string `json:"errors,omitempty"`
}

// config defines how tags should be modified
type config struct {
	file     string
	output   string
	write    bool
	modified io.Reader

	offset     int
	structName string
	line       string
	start, end int

	fset *token.FileSet

	remove        []string
	removeOptions []string

	add                  []string
	addOptions           []string
	override             bool
	skipUnexportedFields bool

	transform   string
	sort        bool
	clear       bool
	clearOption bool
}

func main() {
	if err := realMain(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func realMain() error {
	var (
		// file flags
		flagFile  = flag.String("file", "", "Filename to be parsed")
		flagWrite = flag.Bool("w", false,
			"Write result to (source) file instead of stdout")
		flagOutput = flag.String("format", "source", "Output format."+
			"By default it's the whole file. Options: [source, json]")
		flagModified = flag.Bool("modified", false, "read an archive of modified files from standard input")

		// processing modes
		flagOffset = flag.Int("offset", 0,
			"Byte offset of the cursor position inside a struct."+
				"Can be anwhere from the comment until closing bracket")
		flagLine = flag.String("line", "",
			"Line number of the field or a range of line. i.e: 4 or 4,8")
		flagStruct = flag.String("struct", "", "Struct name to be processed")

		// tag flags
		flagRemoveTags = flag.String("remove-tags", "",
			"Remove tags for the comma separated list of keys")
		flagClearTags = flag.Bool("clear-tags", false,
			"Clear all tags")
		flagAddTags = flag.String("add-tags", "",
			"Adds tags for the comma separated list of keys."+
				"Keys can contain a static value, i,e: json:foo")
		flagOverride          = flag.Bool("override", false, "Override current tags when adding tags")
		flagSkipPrivateFields = flag.Bool("skip-unexported", false, "Skip unexported fields")
		flagTransform         = flag.String("transform", "snakecase",
			"Transform adds a transform rule when adding tags."+
				" Current options: [snakecase, camelcase, lispcase, pascalcase, keep]")
		flagSort = flag.Bool("sort", false,
			"Sort sorts the tags in increasing order according to the key name")

		// option flags
		flagRemoveOptions = flag.String("remove-options", "",
			"Remove the comma separated list of options from the given keys, "+
				"i.e: json=omitempty,hcl=squash")
		flagClearOptions = flag.Bool("clear-options", false,
			"Clear all tag options")
		flagAddOptions = flag.String("add-options", "",
			"Add the options per given key. i.e: json=omitempty,hcl=squash")
	)

	// don't output full help information if something goes wrong
	flag.Usage = func() {}
	flag.Parse()

	if flag.NFlag() == 0 {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		return nil
	}

	cfg := &config{
		file:                 *flagFile,
		line:                 *flagLine,
		structName:           *flagStruct,
		offset:               *flagOffset,
		output:               *flagOutput,
		write:                *flagWrite,
		clear:                *flagClearTags,
		clearOption:          *flagClearOptions,
		transform:            *flagTransform,
		sort:                 *flagSort,
		override:             *flagOverride,
		skipUnexportedFields: *flagSkipPrivateFields,
	}

	if *flagModified {
		cfg.modified = os.Stdin
	}

	if *flagAddTags != "" {
		cfg.add = strings.Split(*flagAddTags, ",")
	}

	if *flagAddOptions != "" {
		cfg.addOptions = strings.Split(*flagAddOptions, ",")
	}

	if *flagRemoveTags != "" {
		cfg.remove = strings.Split(*flagRemoveTags, ",")
	}

	if *flagRemoveOptions != "" {
		cfg.removeOptions = strings.Split(*flagRemoveOptions, ",")
	}

	err := cfg.validate()
	if err != nil {
		return err
	}

	node, err := cfg.parse()
	if err != nil {
		return err
	}

	start, end, err := cfg.findSelection(node)
	if err != nil {
		return err
	}

	rewrittenNode, errs := cfg.rewrite(node, start, end)
	if errs != nil {
		if _, ok := errs.(*rewriteErrors); !ok {
			return errs
		}
	}

	out, err := cfg.format(rewrittenNode, errs)
	if err != nil {
		return err
	}

	fmt.Println(out)
	return nil
}

func (c *config) parse() (ast.Node, error) {
	c.fset = token.NewFileSet()
	var contents interface{}
	if c.modified != nil {
		archive, err := buildutil.ParseOverlayArchive(c.modified)
		if err != nil {
			return nil, fmt.Errorf("failed to parse -modified archive: %v", err)
		}
		fc, ok := archive[c.file]
		if !ok {
			return nil, fmt.Errorf("couldn't find %s in archive", c.file)
		}
		contents = fc
	}

	return parser.ParseFile(c.fset, c.file, contents, parser.ParseComments)
}

// findSelection returns the start and end position of the fields that are
// suspect to change. It depends on the line, struct or offset selection.
func (c *config) findSelection(node ast.Node) (int, int, error) {
	if c.line != "" {
		return c.lineSelection(node)
	} else if c.offset != 0 {
		return c.offsetSelection(node)
	} else if c.structName != "" {
		return c.structSelection(node)
	} else {
		return 0, 0, errors.New("-line, -offset or -struct is not passed")
	}
}

func (c *config) process(fieldName, tagVal string) (string, error) {
	var tag string
	if tagVal != "" {
		var err error
		tag, err = strconv.Unquote(tagVal)
		if err != nil {
			return "", err
		}
	}

	tags, err := structtag.Parse(tag)
	if err != nil {
		return "", err
	}

	tags = c.removeTags(tags)
	tags, err = c.removeTagOptions(tags)
	if err != nil {
		return "", err
	}

	tags = c.clearTags(tags)
	tags = c.clearOptions(tags)

	tags, err = c.addTags(fieldName, tags)
	if err != nil {
		return "", err
	}

	tags, err = c.addTagOptions(tags)
	if err != nil {
		return "", err
	}

	if c.sort {
		sort.Sort(tags)
	}

	res := tags.String()
	if res != "" {
		res = quote(tags.String())
	}

	return res, nil
}

func (c *config) removeTags(tags *structtag.Tags) *structtag.Tags {
	if c.remove == nil || len(c.remove) == 0 {
		return tags
	}

	tags.Delete(c.remove...)
	return tags
}

func (c *config) clearTags(tags *structtag.Tags) *structtag.Tags {
	if !c.clear {
		return tags
	}

	tags.Delete(tags.Keys()...)
	return tags
}

func (c *config) clearOptions(tags *structtag.Tags) *structtag.Tags {
	if !c.clearOption {
		return tags
	}

	for _, t := range tags.Tags() {
		t.Options = nil
	}

	return tags
}

func (c *config) removeTagOptions(tags *structtag.Tags) (*structtag.Tags, error) {
	if c.removeOptions == nil || len(c.removeOptions) == 0 {
		return tags, nil
	}

	for _, val := range c.removeOptions {
		// syntax key=option
		splitted := strings.Split(val, "=")
		if len(splitted) != 2 {
			return nil, errors.New("wrong syntax to remove an option. i.e key=option")
		}

		key := splitted[0]
		option := splitted[1]

		tags.DeleteOptions(key, option)
	}

	return tags, nil
}

func (c *config) addTagOptions(tags *structtag.Tags) (*structtag.Tags, error) {
	if c.addOptions == nil || len(c.addOptions) == 0 {
		return tags, nil
	}

	for _, val := range c.addOptions {
		// syntax key=option
		splitted := strings.Split(val, "=")
		if len(splitted) != 2 {
			return nil, errors.New("wrong syntax to add an option. i.e key=option")
		}

		key := splitted[0]
		option := splitted[1]

		tags.AddOptions(key, option)
	}

	return tags, nil
}

func (c *config) addTags(fieldName string, tags *structtag.Tags) (*structtag.Tags, error) {
	if c.add == nil || len(c.add) == 0 {
		return tags, nil
	}

	splitted := camelcase.Split(fieldName)
	name := ""

	unknown := false
	switch c.transform {
	case "snakecase":
		var lowerSplitted []string
		for _, s := range splitted {
			lowerSplitted = append(lowerSplitted, strings.ToLower(s))
		}

		name = strings.Join(lowerSplitted, "_")
	case "lispcase":
		var lowerSplitted []string
		for _, s := range splitted {
			lowerSplitted = append(lowerSplitted, strings.ToLower(s))
		}

		name = strings.Join(lowerSplitted, "-")
	case "camelcase":
		var titled []string
		for _, s := range splitted {
			titled = append(titled, strings.Title(s))
		}

		titled[0] = strings.ToLower(titled[0])

		name = strings.Join(titled, "")
	case "pascalcase":
		var titled []string
		for _, s := range splitted {
			titled = append(titled, strings.Title(s))
		}

		name = strings.Join(titled, "")
	case "keep":
		name = fieldName
	default:
		unknown = true
	}

	for _, key := range c.add {
		splitted = strings.Split(key, ":")
		if len(splitted) == 2 {
			key = splitted[0]
			name = splitted[1]
		} else if unknown {
			// the user didn't pass any value but want to use an unknown
			// transform. We don't return above in the default as the user
			// might pass a value
			return nil, fmt.Errorf("unknown transform option %q", c.transform)
		}

		tag, err := tags.Get(key)
		if err != nil {
			// tag doesn't exist, create a new one
			tag = &structtag.Tag{
				Key:  key,
				Name: name,
			}
		} else if c.override {
			tag.Name = name
		}

		if err := tags.Set(tag); err != nil {
			return nil, err
		}
	}

	return tags, nil
}

// collectStructs collects and maps structType nodes to their positions
func collectStructs(node ast.Node) map[token.Pos]*structType {
	structs := make(map[token.Pos]*structType, 0)
	collectStructs := func(n ast.Node) bool {
		var t ast.Expr
		var structName string

		switch x := n.(type) {
		case *ast.TypeSpec:
			if x.Type == nil {
				return true

			}

			structName = x.Name.Name
			t = x.Type
		case *ast.CompositeLit:
			t = x.Type
		case *ast.ValueSpec:
			structName = x.Names[0].Name
			t = x.Type
		}

		x, ok := t.(*ast.StructType)
		if !ok {
			return true
		}

		structs[x.Pos()] = &structType{
			name: structName,
			node: x,
		}
		return true
	}
	ast.Inspect(node, collectStructs)
	return structs
}

func (c *config) format(file ast.Node, rwErrs error) (string, error) {
	switch c.output {
	case "source":
		var buf bytes.Buffer
		err := format.Node(&buf, c.fset, file)
		if err != nil {
			return "", err
		}

		if c.write {
			err = ioutil.WriteFile(c.file, buf.Bytes(), 0)
			if err != nil {
				return "", err
			}
		}

		return buf.String(), nil
	case "json":
		// NOTE(arslan): print first the whole file and then cut out our
		// selection. The reason we don't directly print the struct is that the
		// printer is not capable of printing loosy comments, comments that are
		// not part of any field inside a struct. Those are part of *ast.File
		// and only printed inside a struct if we print the whole file. This
		// approach is the sanest and simplest way to get a struct printed
		// back. Second, our cursor might intersect two different structs with
		// other declarations in between them. Printing the file and cutting
		// the selection is the easier and simpler to do.
		var buf bytes.Buffer

		// this is the default config from `format.Node()`, but we add
		// `printer.SourcePos` to get the original source position of the
		// modified lines
		cfg := printer.Config{Mode: printer.SourcePos | printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
		err := cfg.Fprint(&buf, c.fset, file)
		if err != nil {
			return "", err
		}

		lines, err := parseLines(&buf)
		if err != nil {
			return "", err
		}

		// prevent selection to be larger than the actual number of lines
		if c.start > len(lines) || c.end > len(lines) {
			return "", errors.New("line selection is invalid")
		}

		out := &output{
			Start: c.start,
			End:   c.end,
			Lines: lines[c.start-1 : c.end],
		}

		if rwErrs != nil {
			if r, ok := rwErrs.(*rewriteErrors); ok {
				for _, err := range r.errs {
					out.Errors = append(out.Errors, err.Error())
				}
			}
		}

		o, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return "", err
		}

		return string(o), nil
	default:
		return "", fmt.Errorf("unknown output mode: %s", c.output)
	}
}

func (c *config) lineSelection(file ast.Node) (int, int, error) {
	var err error
	splitted := strings.Split(c.line, ",")

	start, err := strconv.Atoi(splitted[0])
	if err != nil {
		return 0, 0, err
	}

	end := start
	if len(splitted) == 2 {
		end, err = strconv.Atoi(splitted[1])
		if err != nil {
			return 0, 0, err
		}
	}

	if start > end {
		return 0, 0, errors.New("wrong range. start line cannot be larger than end line")
	}

	return start, end, nil
}

func (c *config) structSelection(file ast.Node) (int, int, error) {
	structs := collectStructs(file)

	var encStruct *ast.StructType
	for _, st := range structs {
		if st.name == c.structName {
			encStruct = st.node
		}
	}

	if encStruct == nil {
		return 0, 0, errors.New("struct name does not exist")
	}

	// struct selects all lines inside a struct
	start := c.fset.Position(encStruct.Pos()).Line
	end := c.fset.Position(encStruct.End()).Line

	return start, end, nil
}

func (c *config) offsetSelection(file ast.Node) (int, int, error) {
	structs := collectStructs(file)

	var encStruct *ast.StructType
	for _, st := range structs {
		structBegin := c.fset.Position(st.node.Pos()).Offset
		structEnd := c.fset.Position(st.node.End()).Offset

		if structBegin <= c.offset && c.offset <= structEnd {
			encStruct = st.node
			break
		}
	}

	if encStruct == nil {
		return 0, 0, errors.New("offset is not inside a struct")
	}

	// offset selects all fields
	start := c.fset.Position(encStruct.Pos()).Line
	end := c.fset.Position(encStruct.End()).Line

	return start, end, nil
}

func isPublicName(name string) bool {
	for _, c := range name {
		return unicode.IsUpper(c)
	}
	return false
}

// rewrite rewrites the node for structs between the start and end
// positions
func (c *config) rewrite(node ast.Node, start, end int) (ast.Node, error) {
	errs := &rewriteErrors{errs: make([]error, 0)}

	rewriteFunc := func(n ast.Node) bool {
		x, ok := n.(*ast.StructType)
		if !ok {
			return true
		}

		for _, f := range x.Fields.List {
			line := c.fset.Position(f.Pos()).Line

			if !(start <= line && line <= end) {
				continue
			}

			fieldName := ""
			if len(f.Names) != 0 {
				for _, field := range f.Names {
					if !c.skipUnexportedFields || isPublicName(field.Name) {
						fieldName = field.Name
						break
					}
				}

				// nothing to process, continue with next line
				if fieldName == "" {
					continue
				}
			}

			// anonymous field
			if f.Names == nil {
				ident, ok := f.Type.(*ast.Ident)
				if !ok {
					continue
				}

				fieldName = ident.Name
			}

			if f.Tag == nil {
				f.Tag = &ast.BasicLit{}
			}

			res, err := c.process(fieldName, f.Tag.Value)
			if err != nil {
				errs.Append(fmt.Errorf("%s:%d:%d:%s",
					c.fset.Position(f.Pos()).Filename,
					c.fset.Position(f.Pos()).Line,
					c.fset.Position(f.Pos()).Column,
					err))
				continue
			}

			f.Tag.Value = res
		}

		return true
	}

	ast.Inspect(node, rewriteFunc)

	c.start = start
	c.end = end

	if len(errs.errs) == 0 {
		return node, nil
	}

	return node, errs
}

// validate validates whether the config is valid or not
func (c *config) validate() error {
	if c.file == "" {
		return errors.New("no file is passed")
	}

	if c.line == "" && c.offset == 0 && c.structName == "" {
		return errors.New("-line, -offset or -struct is not passed")
	}

	if c.line != "" && c.offset != 0 ||
		c.line != "" && c.structName != "" ||
		c.offset != 0 && c.structName != "" {
		return errors.New("-line, -offset or -struct cannot be used together. pick one")
	}

	if (c.add == nil || len(c.add) == 0) &&
		(c.addOptions == nil || len(c.addOptions) == 0) &&
		!c.clear &&
		!c.clearOption &&
		(c.removeOptions == nil || len(c.removeOptions) == 0) &&
		(c.remove == nil || len(c.remove) == 0) {
		return errors.New("one of " +
			"[-add-tags, -add-options, -remove-tags, -remove-options, -clear-tags, -clear-options]" +
			" should be defined")
	}

	return nil
}

func quote(tag string) string {
	return "`" + tag + "`"
}

type rewriteErrors struct {
	errs []error
}

func (r *rewriteErrors) Error() string {
	var buf bytes.Buffer
	for _, e := range r.errs {
		buf.WriteString(fmt.Sprintf("%s\n", e.Error()))
	}
	return buf.String()
}

func (r *rewriteErrors) Append(err error) {
	if err == nil {
		return
	}

	r.errs = append(r.errs, err)
}

// parseLines parses the given buffer and returns a slice of lines
func parseLines(buf io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		txt := scanner.Text()

		// check for any line directive and store it for next iteration to
		// re-construct the original file. If it's not a line directive,
		// continue consturcting the original file
		if !strings.HasPrefix(txt, "//line") {
			lines = append(lines, txt)
			continue
		}

		lineNr, err := split(txt)
		if err != nil {
			return nil, err
		}

		for i := len(lines); i < lineNr-1; i++ {
			lines = append(lines, "")
		}

		lines = lines[:lineNr-1]
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("invalid scanner inputl: %s", err)
	}

	return lines, nil
}

// split splits the given line directive and returns the line number
// see https://golang.org/cmd/compile/#hdr-Compiler_Directives for more
// information
// NOTE(arslan): this only splits the line directive that the go.Parser
// outputs. If the go parser changes the format of the line directive, make
// sure to fix it in the below function
func split(line string) (int, error) {
	for i := len(line) - 1; i >= 0; i-- {
		if line[i] != ':' {
			continue
		}

		nr, err := strconv.Atoi(line[i+1:])
		if err != nil {
			return 0, err
		}

		return nr, nil
	}

	return 0, fmt.Errorf("couldn't parse line: '%s'", line)
}
