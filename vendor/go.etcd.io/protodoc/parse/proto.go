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

import "sort"

// Proto represents sets of 'ProtoMessage' and 'ProtoService'.
type Proto struct {
	Services []ProtoService
	Messages []ProtoMessage
}

type services []ProtoService

func (s services) Len() int           { return len(s) }
func (s services) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s services) Less(i, j int) bool { return s[i].Name < s[j].Name }

type messages []ProtoMessage

func (s messages) Len() int           { return len(s) }
func (s messages) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s messages) Less(i, j int) bool { return s[i].Name < s[j].Name }

func (p *Proto) Sort() {
	sort.Sort(services(p.Services))
	sort.Sort(messages(p.Messages))
}

// ProtoService represents the 'service' type in Protocol Buffer.
// (https://developers.google.com/protocol-buffers/docs/proto3#services)
type ProtoService struct {
	FilePath    string
	Name        string
	Description string
	Methods     []ProtoMethod
}

// ProtoMethod represents methods in ProtoService.
type ProtoMethod struct {
	Name         string
	Description  string
	RequestType  string
	ResponseType string
}

// ProtoMessage represents the 'message' type in Protocol Buffer.
// (https://developers.google.com/protocol-buffers/docs/proto3#simple)
type ProtoMessage struct {
	FilePath    string
	Name        string
	Description string
	Fields      []ProtoField
}

// ProtoField represent member fields in ProtoMessage.
type ProtoField struct {
	Name                 string
	Description          string
	Repeated             bool
	ProtoType            ProtoType
	UserDefinedProtoType string
}
