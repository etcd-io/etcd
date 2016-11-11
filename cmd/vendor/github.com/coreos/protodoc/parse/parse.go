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

package parse

import (
	"bufio"
	"io"
	"os"
	"strings"
)

func readLines(r io.Reader) ([]string, error) {
	var (
		lines   []string
		scanner = bufio.NewScanner(r)
	)
	for scanner.Scan() {
		// remove indentation
		line := strings.TrimSpace(scanner.Text())

		// skip empty line
		if len(line) > 0 {
			if strings.HasPrefix(line, "//") {
				lines = append(lines, line)
				continue
			}

			// remove semi-colon line-separator
			sl := strings.Split(line, ";")
			for _, txt := range sl {
				txt = strings.TrimSpace(txt)
				if len(txt) == 0 {
					continue
				}

				if strings.HasPrefix(txt, "optional ") { // proto2
					txt = strings.Replace(txt, "optional ", "", 1)
				}
				lines = append(lines, txt)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

type parseMode int

const (
	reading parseMode = iota
	parsingMessage
	parsingService
	parsingRPC
)

func ReadDir(targetDir, messageOnlyFromThisFile string) (*Proto, error) {
	rm, err := walkDirExt(targetDir, ".proto")
	if err != nil {
		return nil, err
	}

	pr := &Proto{
		Services: []ProtoService{},
		Messages: []ProtoMessage{},
	}
	for _, fpath := range rm {
		p, err := ReadFile(fpath)
		if err != nil {
			return nil, err
		}
		pr.Services = append(pr.Services, p.Services...)
		if messageOnlyFromThisFile == "" ||
			(messageOnlyFromThisFile != "" && strings.HasSuffix(fpath, messageOnlyFromThisFile)) {
			pr.Messages = append(pr.Messages, p.Messages...)
		}
	}
	return pr, nil
}

func ReadFile(fpath string) (*Proto, error) {
	f, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
	if err != nil {
		return nil, err
	}
	var (
		wd, _ = os.Getwd()
		fs    = fpath
	)
	if strings.HasPrefix(fs, wd) {
		fs = strings.Replace(fs, wd, "", 1)
		if strings.HasPrefix(fs, "/") {
			fs = strings.Replace(fs, "/", "", 1)
		}
	}

	lines, err := readLines(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	f.Close()

	var (
		rp = Proto{
			Services: []ProtoService{},
			Messages: []ProtoMessage{},
		}
		mode = reading

		comments     []string
		protoMessage = ProtoMessage{}
		protoService = ProtoService{}
	)

	skippingEnum := false
	skippingGatewayCnt := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "//") {
			ls := strings.Replace(line, "//", "", 1)
			comments = append(comments, strings.TrimSpace(ls))
			continue
		}

		// skippin enum
		if strings.HasPrefix(line, "enum ") {
			skippingEnum = true
			continue
		}
		if skippingEnum && strings.HasSuffix(line, "}") { // end of enum
			skippingEnum = false
			continue
		}
		if skippingEnum {
			continue
		}

		// skip gengo/grpc-gateway
		if strings.HasPrefix(line, "option (google.api.http) = ") {
			skippingGatewayCnt++
			continue
		}
		if strings.HasPrefix(line, "post: ") {
			skippingGatewayCnt++
			continue
		}
		if strings.HasPrefix(line, "body: ") {
			skippingGatewayCnt++
			continue
		}
		if skippingGatewayCnt == 3 && line == "}" {
			skippingGatewayCnt++
			continue
		}
		if skippingGatewayCnt == 4 {
			skippingGatewayCnt = 0
			continue // end of grpc-gateway
		}

		switch mode {
		case reading:
			for j, elem := range strings.Fields(line) {
				switch j {
				case 0:
					switch elem {
					case "message":
						mode = parsingMessage

					case "service":
						mode = parsingService
					}

				case 1: // proto message/service name
					switch mode {
					case parsingMessage: // message Name
						protoMessage.Name = strings.Replace(elem, "{", "", -1)
						protoMessage.Description = strings.Join(comments, " ")
						comments = []string{}                // reset
						protoMessage.Fields = []ProtoField{} // reset

					case parsingService: // service Name
						protoService.Name = strings.Replace(elem, "{", "", -1)
						protoService.Description = strings.Join(comments, " ")
						comments = []string{}                  // reset
						protoService.Methods = []ProtoMethod{} // reset
					}
				}
			}

		case parsingMessage:
			if strings.HasSuffix(line, "}") { // closing of message
				protoMessage.FilePath = fs
				rp.Messages = append(rp.Messages, protoMessage)
				protoMessage = ProtoMessage{}
				comments = []string{}
				mode = reading
				continue
			}

			protoField := ProtoField{}
			tl := line
			if strings.HasPrefix(tl, "repeated") {
				protoField.Repeated = true
				tl = strings.Replace(tl, "repeated ", "", -1)
			}
			fds := strings.Fields(tl)
			tp, err := ToProtoType(fds[0])
			if err != nil {
				protoField.ProtoType = 0
				protoField.UserDefinedProtoType = fds[0]
			} else {
				protoField.ProtoType = tp
				protoField.UserDefinedProtoType = ""
			}

			protoField.Name = fds[1]
			protoField.Description = strings.Join(comments, " ")
			protoMessage.Fields = append(protoMessage.Fields, protoField)
			comments = []string{}

		case parsingService:
			// parse 'rpc Watch(stream WatchRequest) returns (stream WatchResponse) {}'
			if strings.HasPrefix(line, "rpc ") {
				lt := strings.Replace(line, "rpc ", "", 1)
				lt = strings.Replace(lt, ")", "", -1)
				lt = strings.Replace(lt, " {}", "", 1) // without grpc gateway
				lt = strings.Replace(lt, " {", "", 1)  // with grpc gateway
				fsigs := strings.Split(lt, " returns ")

				ft := strings.Split(fsigs[0], "(") // split 'Watch(stream WatchRequest'
				f1 := ft[0]

				ft = strings.Fields(ft[1])
				f2 := ft[len(ft)-1]

				ft = strings.Fields(strings.Replace(fsigs[1], "(", "", 1)) // split '(stream WatchResponse'
				f3 := ft[len(ft)-1]

				protoMethod := ProtoMethod{} // reset
				protoMethod.Name = f1
				protoMethod.RequestType = f2
				protoMethod.ResponseType = f3
				protoMethod.Description = strings.Join(comments, " ")
				protoService.Methods = append(protoService.Methods, protoMethod)
				comments = []string{}
			} else if !strings.HasSuffix(line, "{}") && strings.HasSuffix(line, "}") {
				// end of service
				protoService.FilePath = fs
				rp.Services = append(rp.Services, protoService)
				protoService = ProtoService{}
				comments = []string{}
				mode = reading
			}
		}
	}

	return &rp, nil
}
