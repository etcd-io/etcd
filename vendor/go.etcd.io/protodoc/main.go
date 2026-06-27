// Copyright 2016 CoreOS, Inc.
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

// protodoc generates Protocol Buffer documentation.
//
//	Usage:
//	protodoc [flags]
//
//	Flags:
//		--directories=: comma separated map of target directory to parse options (e.g. 'dirA=message,dirB=message_service')
//	-d, --directory="": target directory where Protocol Buffer files are.
//	-c, --disclaimer="": disclaimer statement
//	-h, --help[=false]: help for protodoc
//	-l, --languages=[]: language options in field descriptions (Go, C++, Java, Python, Ruby, C#)
//		--message-only-from-this-file="": if specified, it parses only the messages in this file within the directory
//	-o, --output="": output file path to save documentation
//	-p, --parse=[service,message]: Protocol Buffer types to parse (message, service)
//	-t, --title="": title of documentation
//
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.etcd.io/protodoc/parse"

	"github.com/spf13/cobra"
)

var (
	rootCommand = &cobra.Command{
		Use:   "protodoc",
		Short: "protodoc generates Protocol Buffer documentation.",
		RunE:  CommandFunc,
	}

	targetDirectory string
	parseOptions    []string
	languageOptions []string
	title           string
	outputPath      string
	disclaimer      string

	targetDirectories       = newDirectoryOptions()
	messageOnlyFromThisFile string
)

type directoryOption struct {
	directory string
	options   []parse.ParseOption
}

type directoryOptions []directoryOption

func newDirectoryOptions() directoryOptions {
	return directoryOptions(make([]directoryOption, 0))
}

func (do directoryOptions) String() string {
	return ""
}

func (do *directoryOptions) Set(s string) error {
	for _, elem := range strings.Split(s, ",") {
		pair := strings.Split(elem, "=")
		if len(pair) != 2 {
			return fmt.Errorf("invalid format %s", pair)
		}
		opts := []parse.ParseOption{}
		for _, v := range strings.Split(pair[1], "_") {
			switch v {
			case "message":
				opts = append(opts, parse.ParseMessage)
			case "service":
				opts = append(opts, parse.ParseService)
			}
		}
		*do = append(*do, directoryOption{directory: pair[0], options: opts})
	}
	return nil
}

func (do directoryOptions) Type() string {
	return "[]directoryOption"
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCommand.PersistentFlags().StringVarP(&targetDirectory, "directory", "d", "", "target directory where Protocol Buffer files are.")
	rootCommand.PersistentFlags().StringSliceVarP(&parseOptions, "parse", "p", []string{"service", "message"}, "Protocol Buffer types to parse (message, service)")
	rootCommand.PersistentFlags().StringSliceVarP(&languageOptions, "languages", "l", []string{}, "language options in field descriptions (Go, C++, Java, Python, Ruby, C#)")
	rootCommand.PersistentFlags().StringVarP(&title, "title", "t", "", "title of documentation")
	rootCommand.PersistentFlags().StringVarP(&outputPath, "output", "o", "", "output file path to save documentation")
	rootCommand.PersistentFlags().StringVarP(&disclaimer, "disclaimer", "c", "", "disclaimer statement")

	rootCommand.PersistentFlags().Var(&targetDirectories, "directories", "comma separated map of target directory to parse options (e.g. 'dirA=message,dirB=message_service')")
	rootCommand.PersistentFlags().StringVar(&messageOnlyFromThisFile, "message-only-from-this-file", "", "if specified, it parses only the messages in this file within the directory")
}

func CommandFunc(cmd *cobra.Command, args []string) error {
	var rs string
	if len(disclaimer) > 0 {
		rs += disclaimer + "\n\n\n"
	}
	if len(targetDirectories) == 0 {
		log.Println("opening", targetDirectory)
		proto, err := parse.ReadDir(targetDirectory, "")
		if err != nil {
			return err
		}
		opts := []parse.ParseOption{}
		for _, v := range parseOptions {
			switch v {
			case "message":
				opts = append(opts, parse.ParseMessage)
			case "service":
				opts = append(opts, parse.ParseService)
			}
		}
		log.Println("converting to markdown", title)
		rs, err = proto.Markdown(title, opts, languageOptions...)
		if err != nil {
			return err
		}
	} else {
		for _, elem := range targetDirectories {
			log.Println("opening", elem.directory)
			c1 := filepath.Base(filepath.Dir(messageOnlyFromThisFile))
			c2 := filepath.Base(elem.directory)
			bs := ""
			if c1 == c2 {
				bs = messageOnlyFromThisFile
				log.Println("message only from this file:", messageOnlyFromThisFile)
			}
			proto, err := parse.ReadDir(elem.directory, bs)
			if err != nil {
				return err
			}
			ms, err := proto.Markdown("", elem.options, languageOptions...)
			if err != nil {
				return err
			}
			rs += ms
		}
		rs = fmt.Sprintf("### %s\n\n\n", title) + rs
	}
	err := toFile(rs, outputPath)
	if err != nil {
		return err
	}
	log.Printf("saved at %s", outputPath)
	return nil
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
}

func toFile(txt, fpath string) error {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(fpath)
		if err != nil {
			return err
		}
	}
	defer f.Close()
	_, err = f.WriteString(txt)
	return err
}
