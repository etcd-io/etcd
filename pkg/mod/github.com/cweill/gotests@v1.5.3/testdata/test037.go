package testdata

import "github.com/cweill/gotests"

type someIndirectImportedStruct gotests.Options

func (smtg *someIndirectImportedStruct) Foo037() {}
