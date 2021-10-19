// Copyright 2021 The etcd Authors
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
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const (
	EtcdVersionAnnotation = "etcd_version"
)

func main() {
	annotation := flag.String("annotation", "", "Specify what proto annotation to read. Options: etcd_version")
	flag.Parse()
	var errs []error
	switch *annotation {
	case EtcdVersionAnnotation:
		errs = handleEtcdVersion()
	case "":
		fmt.Fprintf(os.Stderr, "Please provide --annotation flag")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "Unknown annotation %q. Options: etcd_version", *annotation)
		os.Exit(1)
	}
	if len(errs) != 0 {
		for _, err := range errs {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func handleEtcdVersion() (errs []error) {
	annotations, err := allEtcdVersionAnnotations()
	if err != nil {
		errs = append(errs, err)
		return errs
	}
	sort.Slice(annotations, func(i, j int) bool {
		return annotations[i].fullName < annotations[j].fullName
	})
	output := &bytes.Buffer{}
	for _, a := range annotations {
		newErrs := a.Validate()
		if len(newErrs) == 0 {
			err := a.PrintLine(output)
			if err != nil {
				errs = append(errs, err)
				return errs
			}
		}
		errs = append(errs, newErrs...)
	}
	if len(errs) == 0 {
		fmt.Print(output)
	}
	return errs
}

func allEtcdVersionAnnotations() (annotations []etcdVersionAnnotation, err error) {
	var fileAnnotations []etcdVersionAnnotation
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		switch string(file.Package()) {
		// Skip external packages that are not expected to have etcd version annotation.
		case "io.prometheus.client", "grpc.binarylog.v1", "google.protobuf", "google.rpc", "google.api":
			return true
		}
		fileAnnotations, err = fileEtcdVersionAnnotations(file)
		if err != nil {
			return false
		}
		annotations = append(annotations, fileAnnotations...)
		return true
	})
	return annotations, err
}

func fileEtcdVersionAnnotations(file protoreflect.FileDescriptor) (annotations []etcdVersionAnnotation, err error) {
	err = visitFileDescriptor(file, func(path string, ver *semver.Version) error {
		a := etcdVersionAnnotation{fullName: path, version: ver}
		annotations = append(annotations, a)
		return nil
	})
	return annotations, err
}

type Visitor func(path string, ver *semver.Version) error

func visitFileDescriptor(file protoreflect.FileDescriptor, visitor Visitor) error {
	msgs := file.Messages()
	for i := 0; i < msgs.Len(); i++ {
		err := visitMessageDescriptor(msgs.Get(i), visitor)
		if err != nil {
			return err
		}
	}
	enums := file.Enums()
	for i := 0; i < enums.Len(); i++ {
		err := visitEnumDescriptor(enums.Get(i), visitor)
		if err != nil {
			return err
		}
	}
	return nil
}

func visitMessageDescriptor(md protoreflect.MessageDescriptor, visitor Visitor) error {
	err := VisitDescriptor(md, visitor)
	if err != nil {
		return err
	}
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		err = VisitDescriptor(fd, visitor)
		if err != nil {
			return err
		}
	}

	enums := md.Enums()
	for i := 0; i < enums.Len(); i++ {
		err := visitEnumDescriptor(enums.Get(i), visitor)
		if err != nil {
			return err
		}
	}
	return err
}

func visitEnumDescriptor(enum protoreflect.EnumDescriptor, visitor Visitor) error {
	err := VisitDescriptor(enum, visitor)
	if err != nil {
		return err
	}
	fields := enum.Values()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		err = VisitDescriptor(fd, visitor)
		if err != nil {
			return err
		}
	}
	return err
}

func VisitDescriptor(md protoreflect.Descriptor, visitor Visitor) error {
	s, ok := md.Options().(fmt.Stringer)
	if !ok {
		return nil
	}
	ver, err := wal.EtcdVersionFromOptionsString(s.String())
	if err != nil {
		return fmt.Errorf("%s: %s", md.FullName(), err)
	}
	return visitor(string(md.FullName()), ver)
}

type etcdVersionAnnotation struct {
	fullName string
	version  *semver.Version
}

func (a etcdVersionAnnotation) Validate() (errs []error) {
	if a.version == nil {
		return nil
	}
	if a.version.Major == 0 {
		errs = append(errs, fmt.Errorf("%s: etcd_version major version should not be zero", a.fullName))
	}
	if a.version.Patch != 0 {
		errs = append(errs, fmt.Errorf("%s: etcd_version patch version should be zero", a.fullName))
	}
	if a.version.PreRelease != "" {
		errs = append(errs, fmt.Errorf("%s: etcd_version should not be prerelease", a.fullName))
	}
	if a.version.Metadata != "" {
		errs = append(errs, fmt.Errorf("%s: etcd_version should not have metadata", a.fullName))
	}
	return errs
}

func (a etcdVersionAnnotation) PrintLine(out io.Writer) error {
	if a.version == nil {
		_, err := fmt.Fprintf(out, "%s: \"\"\n", a.fullName)
		return err
	}
	_, err := fmt.Fprintf(out, "%s: \"%d.%d\"\n", a.fullName, a.version.Major, a.version.Minor)
	return err
}
