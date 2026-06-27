package gci

import (
	"bytes"
	"errors"
	"fmt"
	goFormat "go/format"
	"os"
	"sync"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"golang.org/x/sync/errgroup"

	"github.com/daixiang0/gci/pkg/config"
	"github.com/daixiang0/gci/pkg/format"
	"github.com/daixiang0/gci/pkg/io"
	"github.com/daixiang0/gci/pkg/log"
	"github.com/daixiang0/gci/pkg/parse"
	"github.com/daixiang0/gci/pkg/section"
	"github.com/daixiang0/gci/pkg/utils"
)

func LocalFlagsToSections(localFlags []string) section.SectionList {
	sections := section.DefaultSections()
	// Add all local arguments as ImportPrefix sections
	// for _, l := range localFlags {
	// 	sections = append(sections, section.Section{l, nil, nil})
	// }
	return sections
}

func PrintFormattedFiles(paths []string, cfg config.Config) error {
	return processStdInAndGoFilesInPaths(paths, cfg, func(filePath string, unmodifiedFile, formattedFile []byte) error {
		fmt.Print(string(formattedFile))
		return nil
	})
}

func WriteFormattedFiles(paths []string, cfg config.Config) error {
	return processGoFilesInPaths(paths, cfg, func(filePath string, unmodifiedFile, formattedFile []byte) error {
		if bytes.Equal(unmodifiedFile, formattedFile) {
			log.L().Debug(fmt.Sprintf("Skipping correctly formatted File: %s", filePath))
			return nil
		}
		log.L().Info(fmt.Sprintf("Writing formatted File: %s", filePath))
		return os.WriteFile(filePath, formattedFile, 0o644)
	})
}

func ListUnFormattedFiles(paths []string, cfg config.Config) error {
	return processGoFilesInPaths(paths, cfg, func(filePath string, unmodifiedFile, formattedFile []byte) error {
		if bytes.Equal(unmodifiedFile, formattedFile) {
			return nil
		}
		fmt.Println(filePath)
		return nil
	})
}

func DiffFormattedFiles(paths []string, cfg config.Config) error {
	return processStdInAndGoFilesInPaths(paths, cfg, func(filePath string, unmodifiedFile, formattedFile []byte) error {
		fileURI := span.URIFromPath(filePath)
		edits := myers.ComputeEdits(fileURI, string(unmodifiedFile), string(formattedFile))
		unifiedEdits := gotextdiff.ToUnified(filePath, filePath, string(unmodifiedFile), edits)
		fmt.Printf("%v", unifiedEdits)
		return nil
	})
}

func DiffFormattedFilesToArray(paths []string, cfg config.Config, diffs *[]string, lock *sync.Mutex) error {
	log.InitLogger()
	defer log.L().Sync()
	return processStdInAndGoFilesInPaths(paths, cfg, func(filePath string, unmodifiedFile, formattedFile []byte) error {
		fileURI := span.URIFromPath(filePath)
		edits := myers.ComputeEdits(fileURI, string(unmodifiedFile), string(formattedFile))
		unifiedEdits := gotextdiff.ToUnified(filePath, filePath, string(unmodifiedFile), edits)
		lock.Lock()
		*diffs = append(*diffs, fmt.Sprint(unifiedEdits))
		lock.Unlock()
		return nil
	})
}

type fileFormattingFunc func(filePath string, unmodifiedFile, formattedFile []byte) error

func processStdInAndGoFilesInPaths(paths []string, cfg config.Config, fileFunc fileFormattingFunc) error {
	return ProcessFiles(io.StdInGenerator.Combine(io.GoFilesInPathsGenerator(paths, cfg.SkipVendor)), cfg, fileFunc)
}

func processGoFilesInPaths(paths []string, cfg config.Config, fileFunc fileFormattingFunc) error {
	return ProcessFiles(io.GoFilesInPathsGenerator(paths, cfg.SkipVendor), cfg, fileFunc)
}

func ProcessFiles(fileGenerator io.FileGeneratorFunc, cfg config.Config, fileFunc fileFormattingFunc) error {
	var taskGroup errgroup.Group
	files, err := fileGenerator()
	if err != nil {
		return err
	}
	for _, file := range files {
		// run file processing in parallel
		taskGroup.Go(processingFunc(file, cfg, fileFunc))
	}
	return taskGroup.Wait()
}

func processingFunc(file io.FileObj, cfg config.Config, formattingFunc fileFormattingFunc) func() error {
	return func() error {
		unmodifiedFile, formattedFile, err := LoadFormatGoFile(file, cfg)
		if err != nil {
			// if errors.Is(err, FileParsingError{}) {
			// 	// do not process files that are improperly formatted
			// 	return nil
			// }
			return err
		}
		return formattingFunc(file.Path(), unmodifiedFile, formattedFile)
	}
}

func LoadFormatGoFile(file io.FileObj, cfg config.Config) (src, dist []byte, err error) {
	src, err = file.Load()
	log.L().Debug(fmt.Sprintf("Loaded File: %s", file.Path()))
	if err != nil {
		return nil, nil, err
	}

	return LoadFormat(src, file.Path(), cfg)
}

func LoadFormat(in []byte, path string, cfg config.Config) (src, dist []byte, err error) {
	src = in

	if cfg.SkipGenerated && parse.IsGeneratedFileByComment(string(src)) {
		return src, src, nil
	}

	imports, headEnd, tailStart, cStart, cEnd, err := parse.ParseFile(src, path)
	if err != nil {
		if errors.Is(err, parse.NoImportError{}) {
			return src, src, nil
		}
		return nil, nil, err
	}

	// do not do format if only one import
	if len(imports) <= 1 {
		return src, src, nil
	}

	result, err := format.Format(imports, &cfg)
	if err != nil {
		return nil, nil, err
	}

	firstWithIndex := true

	var body []byte

	// order by section list
	for _, s := range cfg.Sections {
		if len(result[s.String()]) > 0 {
			if len(body) > 0 {
				body = append(body, utils.Linebreak)
			}
			for _, d := range result[s.String()] {
				AddIndent(&body, &firstWithIndex)
				body = append(body, src[d.Start:d.End]...)
			}
		}
	}

	head := make([]byte, headEnd)
	copy(head, src[:headEnd])
	tail := make([]byte, len(src)-tailStart)
	copy(tail, src[tailStart:])

	// ensure C
	if cStart != 0 {
		head = append(head, src[cStart:cEnd]...)
		head = append(head, utils.Linebreak)
	}

	// add beginning of import block
	head = append(head, `import (`...)
	head = append(head, utils.Linebreak)
	// add end of import block
	body = append(body, []byte{utils.RightParenthesis, utils.Linebreak}...)

	log.L().Debug(fmt.Sprintf("head:\n%s", head))
	log.L().Debug(fmt.Sprintf("body:\n%s", body))
	if len(tail) > 20 {
		log.L().Debug(fmt.Sprintf("tail:\n%s", tail[:20]))
	} else {
		log.L().Debug(fmt.Sprintf("tail:\n%s", tail))
	}

	var totalLen int
	slices := [][]byte{head, body, tail}
	for _, s := range slices {
		totalLen += len(s)
	}
	dist = make([]byte, totalLen)
	var i int
	for _, s := range slices {
		i += copy(dist[i:], s)
	}

	// remove ^M(\r\n) from Win to Unix
	dist = bytes.ReplaceAll(dist, []byte{utils.WinLinebreak}, []byte{utils.Linebreak})

	log.L().Debug(fmt.Sprintf("raw:\n%s", dist))
	dist, err = goFormat.Source(dist)
	if err != nil {
		return nil, nil, err
	}

	return src, dist, nil
}

func AddIndent(in *[]byte, first *bool) {
	if *first {
		*first = false
		return
	}
	*in = append(*in, utils.Indent)
}
