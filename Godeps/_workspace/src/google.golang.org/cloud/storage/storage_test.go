// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"testing"
	"time"
)

func TestSignedURL(t *testing.T) {
	expires, _ := time.Parse(time.RFC3339, "2002-10-02T10:00:00-05:00")
	url, err := SignedURL("bucket-name", "object-name", &SignedURLOptions{
		GoogleAccessID: "xxx@clientid",
		PrivateKey:     dummyKey("rsa"),
		Method:         "GET",
		MD5:            []byte("202cb962ac59075b964b07152d234b70"),
		Expires:        expires,
		ContentType:    "application/json",
		Headers:        []string{"x-header1", "x-header2"},
	})
	if err != nil {
		t.Error(err)
	}
	want := "https://storage.googleapis.com/bucket-name/object-name?" +
		"Expires=1033570800&GoogleAccessId=xxx%40clientid&Signature=" +
		"ITqNWQHr7ayIj%2B0Ds5%2FzUT2cWMQQouuFmu6L11Zd3kfNKvm3sjyGIzO" +
		"gZsSUoter1SxP7BcrCzgqIZ9fQmgQnuIpqqLL4kcGmTbKsQS6hTknpJM%2F" +
		"2lS4NY6UH1VXBgm2Tce28kz8rnmqG6svcGvtWuOgJsETeSIl1R9nAEIDCEq" +
		"ZJzoOiru%2BODkHHkpoFjHWAwHugFHX%2B9EX4SxaytiN3oEy48HpYGWV0I" +
		"h8NvU1hmeWzcLr41GnTADeCn7Eg%2Fb5H2GCNO70Cz%2Bw2fn%2BofLCUeR" +
		"YQd%2FhES8oocv5kpHZkstc8s8uz3aKMsMauzZ9MOmGy%2F6VULBgIVvi6a" +
		"AwEBIYOw%3D%3D"
	if url != want {
		t.Fatalf("Unexpected signed URL; found %v", url)
	}
}

func TestSignedURL_PEMPrivateKey(t *testing.T) {
	expires, _ := time.Parse(time.RFC3339, "2002-10-02T10:00:00-05:00")
	url, err := SignedURL("bucket-name", "object-name", &SignedURLOptions{
		GoogleAccessID: "xxx@clientid",
		PrivateKey:     dummyKey("pem"),
		Method:         "GET",
		MD5:            []byte("202cb962ac59075b964b07152d234b70"),
		Expires:        expires,
		ContentType:    "application/json",
		Headers:        []string{"x-header1", "x-header2"},
	})
	if err != nil {
		t.Error(err)
	}
	want := "https://storage.googleapis.com/bucket-name/object-name?" +
		"Expires=1033570800&GoogleAccessId=xxx%40clientid&Signature=" +
		"B7XkS4dfmVDoe%2FoDeXZkWlYmg8u2kI0SizTrzL5%2B9RmKnb5j7Kf34DZ" +
		"JL8Hcjr1MdPFLNg2QV4lEH86Gqgqt%2Fv3jFOTRl4wlzcRU%2FvV5c5HU8M" +
		"qW0FZ0IDbqod2RdsMONLEO6yQWV2HWFrMLKl2yMFlWCJ47et%2BFaHe6v4Z" +
		"EBc0%3D"
	if url != want {
		t.Fatalf("Unexpected signed URL; found %v", url)
	}
}

func TestSignedURL_MissingOptions(t *testing.T) {
	pk := dummyKey("rsa")
	var tests = []struct {
		opts   *SignedURLOptions
		errMsg string
	}{
		{
			&SignedURLOptions{},
			"missing required credentials",
		},
		{
			&SignedURLOptions{GoogleAccessID: "access_id"},
			"missing required credentials",
		},
		{
			&SignedURLOptions{
				GoogleAccessID: "access_id",
				PrivateKey:     pk,
			},
			"missing required method",
		},
		{
			&SignedURLOptions{
				GoogleAccessID: "access_id",
				PrivateKey:     pk,
				Method:         "PUT",
			},
			"missing required expires",
		},
	}
	for _, test := range tests {
		_, err := SignedURL("bucket", "name", test.opts)
		if !strings.Contains(err.Error(), test.errMsg) {
			t.Errorf("expected err: %v, found: %v", test.errMsg, err)
		}
	}
}

func dummyKey(kind string) []byte {
	slurp, err := ioutil.ReadFile(fmt.Sprintf("./testdata/dummy_%s", kind))
	if err != nil {
		log.Fatal(err)
	}
	return slurp
}
