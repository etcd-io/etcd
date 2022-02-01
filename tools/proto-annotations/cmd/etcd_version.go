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

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/server/v3/storage/wal"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var (
	// externalPackages that are not expected to have etcd version annotation.
	externalPackages = []string{"io.prometheus.client", "grpc.binarylog.v1", "google.protobuf", "google.rpc", "google.api"}
)

// printEtcdVersion writes etcd_version proto annotation to stdout and returns any errors encountered when reading annotation.
func printEtcdVersion() []error {
	var errs []error
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
		pkg := string(file.Package())
		for _, externalPkg := range externalPackages {
			if pkg == externalPkg {
				return true
			}
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
	err = wal.VisitFileDescriptor(file, func(path protoreflect.FullName, ver *semver.Version) error {
		a := etcdVersionAnnotation{fullName: path, version: ver}
		annotations = append(annotations, a)
		return nil
	})
	return annotations, err
}

type etcdVersionAnnotation struct {
	fullName protoreflect.FullName
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
